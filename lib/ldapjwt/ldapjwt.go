// Listen to 10389 port for LDAP Request
// and route bind request to the handleBind func
package ldapjwt

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/idemeumjwt"
	ldap "github.com/vjeantet/ldapserver"
)

// https://github.com/vjeantet/ldapserver/blob/v1.0.1/examples/complex/main.go
// https://github.com/vjeantet/ldapserver/blob/v1.0.1/examples/abandon/main.go

type LDAPJwtProxy struct {
	issuer          string
	audience        string
	listenerAddress string
	validator       idemeumjwt.IdemeumJwtValidator
	server          ldap.Server
}

func CreateLDAPJwtProxy(Issuer string, Audience string, ListenerAddr string) LDAPJwtProxy {
	return LDAPJwtProxy{
		issuer:          Issuer,
		audience:        Audience,
		listenerAddress: ListenerAddr,
		validator:       idemeumjwt.NewIdemeumJwtValidator(8 * time.Hour),
	}
}

func (p *LDAPJwtProxy) Start() error {
	ldap.Logger = log.New(os.Stdout, "[LDAPJwt] ", log.LstdFlags)

	//Create a new LDAP Server
	server := ldap.NewServer()

	routes := ldap.NewRouteMux()
	routes.Bind(p.handleBind).Label("Bind")
	routes.NotFound(p.handleNotFound).Label("NotFound")
	routes.Abandon(p.handleAbandon).Label("Abandon")
	routes.Search(p.handleSearch).Label("Search")
	server.Handle(routes)

	log.Printf("[LDAPJwt] Starting LDAPJwtProxy : %v\n", p.listenerAddress)
	go server.ListenAndServe(p.listenerAddress)
	log.Print("[LDAPJwt] Started LDAPJwtProxy...]\n")
	return nil
}

func (p *LDAPJwtProxy) Stop() {
	p.server.Stop()
}

func extractDN(claims *idemeumjwt.IdemeumClaims) string {
	if claims.Email == "" {
		return "CN=" + claims.Subject
	}
	s1 := strings.Split(claims.Email, "@")
	s2 := strings.ReplaceAll(s1[1], ".", ",DC=")
	return "CN=" + s1[0] + ",DC=" + s2
}

func (p *LDAPJwtProxy) handleBind(w ldap.ResponseWriter, m *ldap.Message) {
	r := m.GetBindRequest()
	log.Printf("[LDAPJwt] Bind Request:%v \n", r.Name())
	res := ldap.NewBindResponse(ldap.LDAPResultSuccess)

	if string(r.Name()) == p.audience {
		log.Printf("[LDAPJwt] Service Bind Request, returning success ...\n")
		res.SetResultCode(ldap.LDAPResultSuccess)
		res.SeMatchedDN("CN=" + p.audience)
		w.Write(res)
		return
	}

	jwt := r.AuthenticationSimple().String()
	if jwt == "" {
		res.SetResultCode(ldap.LDAPResultInvalidCredentials)
		res.SetDiagnosticMessage("invalid credentials")
		w.Write(res)
		return
	}

	claims, err := p.validator.ValidateJwtToken(jwt, p.issuer, p.audience)
	if err != nil {
		log.Printf("[LDAPJwt] Failed to validate token :%v \n", jwt)
		res.SetResultCode(ldap.LDAPResultInvalidCredentials)
		res.SetDiagnosticMessage("invalid credentials")
		w.Write(res)
		return
	}

	// transform claims into DN
	res.SeMatchedDN(extractDN(claims))
	w.Write(res)

}

func (p *LDAPJwtProxy) handleNotFound(w ldap.ResponseWriter, r *ldap.Message) {
	switch r.ProtocolOpType() {
	case ldap.ApplicationBindRequest:
		res := ldap.NewBindResponse(ldap.LDAPResultSuccess)
		res.SetDiagnosticMessage("Default binding behavior set to return Success")
		w.Write(res)

	default:
		log.Printf("[LDAPJwt] LDAP Operation Rejection Op[Type: %v, Name: %v] \n", r.ProtocolOpName(), r.ProtocolOpType())
		res := ldap.NewResponse(ldap.LDAPResultUnwillingToPerform)
		res.SetDiagnosticMessage("Operation not implemented by server")
		w.Write(res)
	}
}

func (p *LDAPJwtProxy) handleAbandon(w ldap.ResponseWriter, m *ldap.Message) {
	var req = m.GetAbandonRequest()
	// retreive the request to abandon, and send a abort signal to it
	if requestToAbandon, ok := m.Client.GetMessageByID(int(req)); ok {
		requestToAbandon.Abandon()
		log.Printf("[LDAPJwt] Abandon signal sent to request processor [messageID=%d]\n", int(req))
	}
}

func (p *LDAPJwtProxy) handleSearch(w ldap.ResponseWriter, m *ldap.Message) {
	r := m.GetSearchRequest()
	log.Printf("[LDAPJwt] LDAP Search Request [BaseDn=%s, Filter=%s, FilterString=%s, Attributes=%s, TimeLimit=%d]\n",
		r.BaseObject(), r.Filter(), r.FilterString(), r.Attributes(), r.TimeLimit().Int())

	// Handle Stop Signal (server stop / client disconnected / Abandoned request....)
	select {
	case <-m.Done:
		log.Print("[LDAPJwt] Search Cancelled ....\n")
		return
	default:
	}
	cn := strings.Split(r.FilterString(), "=")[1]
	basedn := string(r.BaseObject())
	e := ldap.NewSearchResultEntry(asCN(basedn, cn))
	w.Write(e)
	res := ldap.NewSearchResultDoneResponse(ldap.LDAPResultSuccess)
	w.Write(res)

}

func asCN(baseDN string, cn string) string {
	if baseDN == "" {
		return "CN=" + cn
	}
	return "CN=" + cn + "," + baseDN
}

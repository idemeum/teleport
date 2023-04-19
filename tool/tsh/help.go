/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"strings"
)

const (
	// loginUsageFooter is printed at the bottom of `tsh help login` output
	loginUsageFooter = `NOTES:
  The --proxy is the url of your Remote Access server. E.g (acme.remote.idemeum.com)

EXAMPLES:
  Will get an authentication token for your user to be able to login to your company's remote servers. 
  $ tsh --proxy=acme.remote.idemeum.com login`

	// missingPrincipalsFooter is printed at the bottom of `tsh ls` when no results are returned.
	missingPrincipalsFooter = `
  Not seeing nodes? Your user may be missing Linux principals. If trying teleport for the first time, follow this guide:

https://goteleport.com/docs/getting-started/linux-server/#step-46-create-a-teleport-user-and-set-up-two-factor-authentication
  `
)

// formatFlagDescription creates the description for the --format flag.
func formatFlagDescription(formats ...string) string {
	return fmt.Sprintf("Format output (%s)", strings.Join(formats, ", "))
}

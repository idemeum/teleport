package publisher

import (
	"context"

	"github.com/cloudflare/cfssl/log"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

type RemoteAccessAppWatcher struct {
	events    types.Events
	publisher AppPublisher
	cfg       remoteAccessAppWatcherConfig
}

type remoteAccessAppWatcherConfig struct {
	Watches []types.WatchKind
}

func NewIdemeumRemoteResourceWatcher(ctx context.Context, events types.Events, appPublisher AppPublisher) *RemoteAccessAppWatcher {
	appWatcher := &RemoteAccessAppWatcher{
		events:    events,
		publisher: appPublisher,
		cfg:       defaultAppWatcherConfig(),
	}
	go appWatcher.watch(ctx)
	return appWatcher
}

// defaultAppWatcherConfig default app watcher config
func defaultAppWatcherConfig() remoteAccessAppWatcherConfig {
	Watches := []types.WatchKind{
		{Kind: types.KindNode},
		{Kind: types.KindAppServer},
		{Kind: types.KindApp},
	}
	return remoteAccessAppWatcherConfig{Watches: Watches}
}

func (c *RemoteAccessAppWatcher) watch(ctx context.Context) error {
	log.Info("Watching the app changes")
	watcher, err := c.events.NewWatcher(ctx, types.Watch{
		Name:  "remote-access-apps",
		Kinds: c.cfg.Watches,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()
	for {
		select {
		case event := <-watcher.Events():
			// OpInit is a special case omitted by the Watcher when the
			// connection succeeds.
			if event.Type == types.OpInit {
				log.Infof("Started watching for apps changes")
				continue
			}
			c.processEvent(event)
		case <-watcher.Done():
			if err := watcher.Error(); err != nil {
				return trace.Wrap(err)
			}
			return nil
		case <-ctx.Done():
			return nil
		}
	}
}

func (c *RemoteAccessAppWatcher) processEvent(event types.Event) error {

	if !canProcessEventType(event) {
		log.Debugf("skipping the event type: %v for resource type: %v and name :%v", event.Type, event.Resource.GetKind(), event.Resource.GetName())
		return nil
	}

	log.Infof("remote access app watcher processing the event type: %v for resource type: %v and name :%v ", event.Type, event.Resource.GetKind(), event.Resource.GetName())
	appType := getRemoteAppType(event.Resource)

	if appType == Invalid {
		log.Debugf("skipping remote access app watcher processing for app type: %v", appType)
		return nil
	}

	return c.publisher.Publish(AppChangeEvent{
		AppType: appType,
	})
}

func canProcessEventType(event types.Event) bool {
	// we are only processing the delete event from the database
	// insert to database are already processed in the place of record insertion
	return event.Type == types.OpDelete
}

func getRemoteAppType(resource types.Resource) RemoteAppType {
	switch resource.GetKind() {
	case types.KindNode:
		return Server
	case types.KindAppServer, types.KindApp:
		return Webapp
	case types.KindWindowsDesktop:
		return Desktop
	}
	return Invalid
}

package signal

import (
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
)

func init() {
	channels.RegisterFactory(
		config.ChannelSignal,
		func(channelName, channelType string, cfg *config.Config, b *bus.MessageBus) (channels.Channel, error) {
			bc := cfg.Channels[channelName]
			decoded, err := bc.GetDecoded()
			if err != nil {
				return nil, err
			}
			c, ok := decoded.(*config.SignalSettings)
			if !ok {
				return nil, channels.ErrSendFailed
			}
			ch, err := NewSignalChannel(bc, c, b)
			if err != nil {
				return nil, err
			}
			if channelName != config.ChannelSignal {
				ch.SetName(channelName)
			}
			return ch, nil
		},
	)
}

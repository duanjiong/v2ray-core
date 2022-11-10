package v2binding

import (
	"github.com/v2fly/v2ray-core/v5/proxy/blackhole"
	"github.com/v2fly/v2ray-core/v5/proxy/freedom"
	"google.golang.org/protobuf/types/known/anypb"

	core "github.com/v2fly/v2ray-core/v5"
	"github.com/v2fly/v2ray-core/v5/app/commander"
	"github.com/v2fly/v2ray-core/v5/app/dispatcher"
	"github.com/v2fly/v2ray-core/v5/app/instman"
	"github.com/v2fly/v2ray-core/v5/app/instman/command"
	"github.com/v2fly/v2ray-core/v5/app/proxyman"
	"github.com/v2fly/v2ray-core/v5/app/router"
	"github.com/v2fly/v2ray-core/v5/common/net"
	"github.com/v2fly/v2ray-core/v5/common/serial"
	_ "github.com/v2fly/v2ray-core/v5/main/distro/all"
	"github.com/v2fly/v2ray-core/v5/proxy/dokodemo"
)

type bindingInstance struct {
	started  bool
	instance *core.Instance
}

var binding bindingInstance

func (b *bindingInstance) startAPIInstance() {
	bindConfig := &core.Config{
		App: []*anypb.Any{
			serial.ToTypedMessage(&instman.Config{}),
			serial.ToTypedMessage(&commander.Config{
				Tag: "api",
				Service: []*anypb.Any{
					serial.ToTypedMessage(&command.Config{}),
				},
			}),
			serial.ToTypedMessage(&router.Config{
				Rule: []*router.RoutingRule{
					{
						InboundTag: []string{"api"},
						TargetTag: &router.RoutingRule_Tag{
							Tag: "api",
						},
					},
				},
			}),
		},
		Inbound: []*core.InboundHandlerConfig{
			{
				Tag: "api",
				ReceiverSettings: serial.ToTypedMessage(&proxyman.ReceiverConfig{
					PortRange: net.SinglePortRange(999),
					Listen:    net.NewIPOrDomain(net.AnyIP),
				}),
				ProxySettings: serial.ToTypedMessage(&dokodemo.Config{
					Address:  net.NewIPOrDomain(net.LocalHostIP),
					Port:     uint32(999),
					Networks: []net.Network{net.Network_TCP},
				}),
			},
			{
				Tag: "from-vpc-to-calico",
				ReceiverSettings: serial.ToTypedMessage(&proxyman.ReceiverConfig{
					PortRange:    net.SinglePortRange(998),
					Listen:       net.NewIPOrDomain(net.AnyIP),
					NetNamespace: "/var/run/netns/test",
				}),
				ProxySettings: serial.ToTypedMessage(&dokodemo.Config{
					Address:           net.NewIPOrDomain(net.LocalHostIP),
					Port:              uint32(997),
					Networks:          []net.Network{net.Network_TCP},
					ForcedOutboundTag: "freedom",
				}),
			},
			{
				Tag: "from-calico-to-vpc",
				ReceiverSettings: serial.ToTypedMessage(&proxyman.ReceiverConfig{
					PortRange: net.SinglePortRange(996),
					Listen:    net.NewIPOrDomain(net.AnyIP),
				}),
				ProxySettings: serial.ToTypedMessage(&dokodemo.Config{
					Address:           net.NewIPOrDomain(net.LocalHostIP),
					Port:              uint32(995),
					Networks:          []net.Network{net.Network_TCP},
					ForcedOutboundTag: "freedom",
					NetNamespace:      "/var/run/netns/test",
				}),
			},
		},
		Outbound: []*core.OutboundHandlerConfig{
			{
				Tag:           "default-outbound",
				ProxySettings: serial.ToTypedMessage(&blackhole.Config{}),
			},
			{
				Tag:           "freedom",
				ProxySettings: serial.ToTypedMessage(&freedom.Config{}),
			},
		},
	}
	bindConfig = withDefaultApps(bindConfig)

	instance, err := core.New(bindConfig)
	if err != nil {
		panic(err)
	}
	err = instance.Start()
	if err != nil {
		panic(err)
	}
	b.instance = instance
}

func withDefaultApps(config *core.Config) *core.Config {
	config.App = append(config.App, serial.ToTypedMessage(&dispatcher.Config{}))
	config.App = append(config.App, serial.ToTypedMessage(&proxyman.InboundConfig{}))
	config.App = append(config.App, serial.ToTypedMessage(&proxyman.OutboundConfig{}))
	return config
}

func StartAPIInstance(basedir string) {
	if binding.started {
		return
	}
	binding.started = true
	binding.startAPIInstance()
}

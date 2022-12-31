package main




import (
	"fmt"
	"strconv"

	iceservers "github.com/OnePlay-Internet/daemon-tool/ice-servers"
	"github.com/OnePlay-Internet/daemon-tool/session"
	proxy "github.com/OnePlay-Internet/webrtc-proxy"
	"github.com/OnePlay-Internet/webrtc-proxy/broadcaster"
	"github.com/OnePlay-Internet/webrtc-proxy/broadcaster/dummy"
	sink "github.com/OnePlay-Internet/webrtc-proxy/broadcaster/gstreamer"
	"github.com/OnePlay-Internet/webrtc-proxy/hid"
	"github.com/OnePlay-Internet/webrtc-proxy/listener"
	"github.com/OnePlay-Internet/webrtc-proxy/listener/audio"
	"github.com/OnePlay-Internet/webrtc-proxy/listener/video"
	"github.com/OnePlay-Internet/webrtc-proxy/signalling"
	"github.com/OnePlay-Internet/webrtc-proxy/util/tool"

	"github.com/OnePlay-Internet/webrtc-proxy/util/config"
	"github.com/pion/webrtc/v3"
)

func main() {
	var token string

	grpcString   := ""
	webrtcString := ""

	HIDURL := "localhost:5000"
	devices := tool.GetDevice()
	if len(devices.Monitors) == 0 {
		fmt.Printf("no display available")
		return
	}

	if token == "" {
		fmt.Printf("no available token")
		return
	}
	

	

	signaling := session.DecodeSignalingConfig(grpcString)
	grpc := &config.GrpcConfig{
		Port:          signaling.Grpcport,
		ServerAddress: signaling.Grpcip,
		Token:         token,
	}

	rtc := &config.WebRTCConfig{ Ices: iceservers.DecodeWebRTCConfig(webrtcString).ICEServers, } ;
	chans := config.NewDataChannelConfig([]string{"hid","adaptive","manual"});
	br    := []*config.BroadcasterConfig{}
	Lists := []listener.Listener{
		audio.CreatePipeline(&config.ListenerConfig{
			StreamID:  "audio",
			Codec:     webrtc.MimeTypeOpus,
		}), video.CreatePipeline(&config.ListenerConfig{
			StreamID:  "video",
			Codec:     webrtc.MimeTypeH264,
		} ,chans.Confs["adaptive"],chans.Confs["manual"]),
	}


	hid.NewHIDSingleton(HIDURL,chans.Confs["hid"])
	prox, err := proxy.InitWebRTCProxy(nil, grpc, rtc, chans,devices, Lists,
		func(tr *webrtc.TrackRemote) (broadcaster.Broadcaster, error) {
			for _, conf := range br {
				if tr.Codec().MimeType == conf.Codec {
					return sink.CreatePipeline(conf)
				} 
			}
			fmt.Printf("no available codec handler, using dummy sink\n")
			return dummy.NewDummyBroadcaster(&config.BroadcasterConfig{
				Name: "dummy",
				Codec:"any",
			})
		},
		func(selection signalling.DeviceSelection) (*tool.MediaDevice,error) {
			monitor := func () tool.Monitor  {
				for _,monitor := range devices.Monitors {
					sel,err := strconv.ParseInt(selection.Monitor,10,32)
					if err != nil {
						return tool.Monitor{}
					}

					if monitor.MonitorHandle == int(sel) {
						return monitor
					}
				}
				return tool.Monitor{MonitorHandle: -1}
			}()
			soundcard := func () tool.Soundcard {
				for _,soundcard := range devices.Soundcards {
					if soundcard.DeviceID == selection.SoundCard {
						return soundcard
					}
				}
				return tool.Soundcard{DeviceID: "none"}
			}()

			for _, listener := range Lists {
				conf := listener.GetConfig()
				if conf.StreamID == "video" {
					err := listener.SetSource(&monitor)
					
					framerate := selection.Framerate;
					if (10 < framerate && framerate < 200) {
						listener.SetProperty("framerate",int(framerate))
					}

					bitrate := selection.Bitrate;
					if (100 < bitrate && bitrate < 20000) {
						listener.SetProperty("bitrate",int(bitrate))
					}

					if err != nil {
						return devices,err
					}

				} else if conf.StreamID == "audio" {
					err := listener.SetSource(&soundcard)
					if err != nil {
						return devices,err
					}
				}
			}
			return nil,nil
		},
	)

	if err != nil {
		fmt.Printf("%s\n", err.Error())
		return
	}
	<-prox.Shutdown
}
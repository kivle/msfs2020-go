package main

// build: GOOS=windows GOARCH=amd64 go build -o simconnect-ws.exe github.com/kivle/msfs2020-go/simconnect-ws

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/kivle/msfs2020-go/simconnect"
	"github.com/kivle/msfs2020-go/simconnect-ws/websockets"
)

type Report struct {
	simconnect.RecvSimobjectDataByType
	Title         [256]byte `name:"TITLE"`
	Altitude      float64   `name:"INDICATED ALTITUDE" unit:"feet"` // PLANE ALTITUDE or PLANE ALT ABOVE GROUND
	Latitude      float64   `name:"PLANE LATITUDE" unit:"degrees"`
	Longitude     float64   `name:"PLANE LONGITUDE" unit:"degrees"`
	Heading       float64   `name:"PLANE HEADING DEGREES TRUE" unit:"degrees"`
	Airspeed      float64   `name:"AIRSPEED INDICATED" unit:"knot"`
	AirspeedTrue  float64   `name:"AIRSPEED TRUE" unit:"knot"`
	VerticalSpeed float64   `name:"VERTICAL SPEED" unit:"ft/min"`
	Flaps         float64   `name:"TRAILING EDGE FLAPS LEFT ANGLE" unit:"degrees"`
	Trim          float64   `name:"ELEVATOR TRIM PCT" unit:"percent"`
	RudderTrim    float64   `name:"RUDDER TRIM PCT" unit:"percent"`
}

func (r *Report) RequestData(s *simconnect.SimConnect) {
	defineID := s.GetDefineID(r)
	requestID := defineID
	s.RequestDataOnSimObjectType(requestID, defineID, 0, simconnect.SIMOBJECT_TYPE_USER)
}

type TrafficReport struct {
	simconnect.RecvSimobjectDataByType
	AtcID           [64]byte `name:"ATC ID"`
	AtcFlightNumber [8]byte  `name:"ATC FLIGHT NUMBER"`
	Altitude        float64  `name:"PLANE ALTITUDE" unit:"feet"`
	Latitude        float64  `name:"PLANE LATITUDE" unit:"degrees"`
	Longitude       float64  `name:"PLANE LONGITUDE" unit:"degrees"`
	Heading         float64  `name:"PLANE HEADING DEGREES TRUE" unit:"degrees"`
}

func (r *TrafficReport) RequestData(s *simconnect.SimConnect) {
	defineID := s.GetDefineID(r)
	requestID := defineID
	s.RequestDataOnSimObjectType(requestID, defineID, 0, simconnect.SIMOBJECT_TYPE_AIRCRAFT)
}

func (r *TrafficReport) Inspect() string {
	return fmt.Sprintf(
		"%s GPS %.6f %.6f @ %.0f feet %.0f°",
		r.AtcID,
		r.Latitude,
		r.Longitude,
		r.Altitude,
		r.Heading,
	)
}

type TeleportRequest struct {
	simconnect.RecvSimobjectDataByType
	Latitude  float64 `name:"PLANE LATITUDE" unit:"degrees"`
	Longitude float64 `name:"PLANE LONGITUDE" unit:"degrees"`
	Altitude  float64 `name:"PLANE ALTITUDE" unit:"feet"`
}

func (r *TeleportRequest) SetData(s *simconnect.SimConnect) {
	defineID := s.GetDefineID(r)

	buf := [3]float64{
		r.Latitude,
		r.Longitude,
		r.Altitude,
	}

	size := simconnect.DWORD(3 * 8) // 2 * 8 bytes
	s.SetDataOnSimObject(defineID, simconnect.OBJECT_ID_USER, 0, 0, size, unsafe.Pointer(&buf[0]))
}

var buildVersion string
var buildTime string
var disableTeleport bool

var verbose bool
var httpListen string
var httpsListen string

func main() {
	flag.BoolVar(&verbose, "verbose", false, "verbose output")
	flag.StringVar(&httpListen, "listen-http", "0.0.0.0:9000", "http listen address (plain HTTP)")
	flag.StringVar(&httpsListen, "listen-https", "0.0.0.0:9443", "https listen address (TLS)")
	flag.BoolVar(&disableTeleport, "disable-teleport", false, "disable teleport")
	flag.Parse()

	fmt.Printf("\nsimconnect-ws (github.com/kivle/msfs2020-go)\n")
	fmt.Printf("readme: https://github.com/kivle/msfs2020-go/blob/master/simconnect-ws/README.md\n")
	fmt.Printf("issues: https://github.com/kivle/msfs2020-go/issues\n")
	fmt.Printf("version: %s (%s)\n", buildVersion, buildTime)
	fmt.Printf("forked from: https://github.com/lian/msfs2020-go/\n\n")

	fmt.Printf("For instructions on how to set up wss:// (TLS), go to http://localhost:9000\n\n")

	fmt.Print("Map application: https://kivle.github.io/msfs-map\n\n")

	exitSignal := make(chan os.Signal, 1)
	signal.Notify(exitSignal, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-exitSignal
		fmt.Print("\n\nExiting...\n")
		os.Exit(0)
	}()

	ws := websockets.New()

	tlsAssets, err := ensureTLSAssets(httpsListen)
	if err != nil {
		panic(fmt.Errorf("prepare TLS assets: %w", err))
	}
	fmt.Printf("TLS enabled; certificate: %s\n", tlsAssets.CertPath)

	go func() {
		mux := http.NewServeMux()

		mux.HandleFunc("/ws", ws.Serve)
		mux.HandleFunc("/cert.pem", certificateDownloadHandler(tlsAssets, "pem"))
		mux.HandleFunc("/cert.der", certificateDownloadHandler(tlsAssets, "der"))
		mux.HandleFunc("/status", statusHandler(httpListen, httpsListen))
		mux.HandleFunc("/", certificateInfoHandler(tlsAssets, httpListen, httpsListen))

		httpServer := &http.Server{Addr: httpListen, Handler: mux}
		httpsServer := &http.Server{Addr: httpsListen, Handler: mux}

		go func() {
			fmt.Printf("HTTP listening on %s\n", httpListen)
			if err := httpServer.ListenAndServe(); err != nil {
				panic(err)
			}
		}()

		fmt.Printf("HTTPS listening on %s\n", httpsListen)
		if err := httpsServer.ListenAndServeTLS(tlsAssets.CertPath, tlsAssets.KeyPath); err != nil {
			panic(err)
		}
	}()

	for {
		mainLoop(exitSignal, ws)
		time.Sleep(time.Second * 5)
	}
}

func mainLoop(exitSignal chan os.Signal, ws *websockets.Websocket) {
	s, err := simconnect.New("simconnect-ws")
	if err != nil {
		if !isIgnorableSimConnectError(err) {
			fmt.Printf("\nFailed to create simconnect connection: %s", err)
		}
		return
	}
	fmt.Println("connected to flight simulator!")
	defer s.Close()

	report := &Report{}
	err = s.RegisterDataDefinition(report)
	if err != nil {
		fmt.Printf("\nFailed to register data definition for Report: %s", err)
		return
	}

	trafficReport := &TrafficReport{}
	err = s.RegisterDataDefinition(trafficReport)
	if err != nil {
		fmt.Printf("\nFailed to register data definition for TrafficReport: %s", err)
		return
	}

	teleportReport := &TeleportRequest{}
	err = s.RegisterDataDefinition(teleportReport)
	if err != nil {
		fmt.Printf("\nFailed to register data definition for TeleportRequest: %s", err)
		return
	}

	eventSimStartID := s.GetEventID()
	//s.SubscribeToSystemEvent(eventSimStartID, "SimStart")
	//s.SubscribeToFacilities(simconnect.FACILITY_LIST_TYPE_AIRPORT, s.GetDefineID(&simconnect.DataFacilityAirport{}))
	//s.SubscribeToFacilities(simconnect.FACILITY_LIST_TYPE_WAYPOINT, s.GetDefineID(&simconnect.DataFacilityWaypoint{}))

	startupTextEventID := s.GetEventID()
	s.ShowText(simconnect.TEXT_TYPE_PRINT_WHITE, 15, startupTextEventID, "simconnect-ws connected")

	simconnectTick := time.NewTicker(100 * time.Millisecond)
	planePositionTick := time.NewTicker(200 * time.Millisecond)
	trafficPositionTick := time.NewTicker(10000 * time.Millisecond)

	for {
		select {
		case <-planePositionTick.C:
			report.RequestData(s)

		case <-trafficPositionTick.C:
			//fmt.Println("--------------------------------- REQUEST TRAFFIC --------------")
			//trafficReport.RequestData(s)
			//s.RequestFacilitiesList(simconnect.FACILITY_LIST_TYPE_AIRPORT, airportRequestID)
			//s.RequestFacilitiesList(simconnect.FACILITY_LIST_TYPE_WAYPOINT, waypointRequestID)

		case <-simconnectTick.C:
			ppData, r1, err := s.GetNextDispatch()
			if err != nil {
				fmt.Printf("simconnect dispatch error: %s (retrying)\n", err)
				return
			}

			if r1 < 0 {
				if uint32(r1) == simconnect.E_FAIL {
					// skip error, means no new messages?
					continue
				} else {
					fmt.Printf("GetNextDispatch error: %d %s (retrying)\n", r1, err)
					return
				}
			}

			recvInfo := *(*simconnect.Recv)(ppData)

			switch recvInfo.ID {
			case simconnect.RECV_ID_EXCEPTION:
				recvErr := *(*simconnect.RecvException)(ppData)
				fmt.Printf("SIMCONNECT_RECV_ID_EXCEPTION %#v\n", recvErr)

			case simconnect.RECV_ID_OPEN:
				recvOpen := *(*simconnect.RecvOpen)(ppData)
				fmt.Printf(
					"\nflight simulator info:\n  codename: %s\n  version: %d.%d (%d.%d)\n  simconnect: %d.%d (%d.%d)\n\n",
					recvOpen.ApplicationName,
					recvOpen.ApplicationVersionMajor,
					recvOpen.ApplicationVersionMinor,
					recvOpen.ApplicationBuildMajor,
					recvOpen.ApplicationBuildMinor,
					recvOpen.SimConnectVersionMajor,
					recvOpen.SimConnectVersionMinor,
					recvOpen.SimConnectBuildMajor,
					recvOpen.SimConnectBuildMinor,
				)

			case simconnect.RECV_ID_EVENT:
				recvEvent := *(*simconnect.RecvEvent)(ppData)

				switch recvEvent.EventID {
				case eventSimStartID:
					fmt.Println("EVENT: SimStart")
				case startupTextEventID:
					// ignore
				default:
					fmt.Println("unknown SIMCONNECT_RECV_ID_EVENT", recvEvent.EventID)
				}
			case simconnect.RECV_ID_WAYPOINT_LIST:
				waypointList := *(*simconnect.RecvFacilityWaypointList)(ppData)
				fmt.Printf("SIMCONNECT_RECV_ID_WAYPOINT_LIST %#v\n", waypointList)

			case simconnect.RECV_ID_AIRPORT_LIST:
				airportList := *(*simconnect.RecvFacilityAirportList)(ppData)
				fmt.Printf("SIMCONNECT_RECV_ID_AIRPORT_LIST %#v\n", airportList)

			case simconnect.RECV_ID_SIMOBJECT_DATA_BYTYPE:
				recvData := *(*simconnect.RecvSimobjectDataByType)(ppData)

				switch recvData.RequestID {
				case s.DefineMap["Report"]:
					report = (*Report)(ppData)

					if verbose {
						fmt.Printf("REPORT: %#v\n", report)
					}

					ws.Broadcast(map[string]interface{}{
						"type":           "plane",
						"latitude":       report.Latitude,
						"longitude":      report.Longitude,
						"altitude":       fmt.Sprintf("%.0f", report.Altitude),
						"heading":        int(report.Heading),
						"airspeed":       fmt.Sprintf("%.0f", report.Airspeed),
						"airspeed_true":  fmt.Sprintf("%.0f", report.AirspeedTrue),
						"vertical_speed": fmt.Sprintf("%.0f", report.VerticalSpeed),
						"flaps":          fmt.Sprintf("%.0f", report.Flaps),
						"trim":           fmt.Sprintf("%.1f", report.Trim),
						"rudder_trim":    fmt.Sprintf("%.1f", report.RudderTrim),
					})

				case s.DefineMap["TrafficReport"]:
					trafficReport = (*TrafficReport)(ppData)
					fmt.Printf("TRAFFIC REPORT: %s\n", trafficReport.Inspect())
				}

			default:
				fmt.Println("recvInfo.ID unknown", recvInfo.ID)
			}

		case <-exitSignal:
			fmt.Println("exiting..")
			os.Exit(0)

		case <-ws.NewConnection:
			// drain and skip

		case m := <-ws.ReceiveMessages:
			handleClientMessage(m, s)
		}
	}
}

func handleClientMessage(m websockets.ReceiveMessage, s *simconnect.SimConnect) {
	var pkt map[string]interface{}
	if err := json.Unmarshal(m.Message, &pkt); err != nil {
		fmt.Println("invalid websocket packet", err)
	} else {
		pktType, ok := pkt["type"].(string)
		if !ok {
			fmt.Println("invalid websocket packet", pkt)
			return
		}
		switch pktType {
		case "teleport":
			if disableTeleport {
				fmt.Println("teleport disabled", pkt)
				return
			}

			// validate user input
			lat, ok := pkt["lat"].(float64)
			if !ok {
				fmt.Println("invalid websocket packet", pkt)
				return
			}
			lng, ok := pkt["lng"].(float64)
			if !ok {
				fmt.Println("invalid websocket packet", pkt)
				return
			}
			altitude, ok := pkt["altitude"].(float64)
			if !ok {
				fmt.Println("invalid websocket packet", pkt)
				return
			}

			// teleport
			r := &TeleportRequest{Latitude: lat, Longitude: lng, Altitude: altitude}
			r.SetData(s)
		}
	}
}

func isIgnorableSimConnectError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "SimConnect_Open error: -2147467259") &&
		strings.Contains(msg, "The operation completed successfully")
}

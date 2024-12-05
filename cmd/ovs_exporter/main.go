package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/greenpau/ovs_exporter/pkg/ovs_exporter"
	ovs "github.com/greenpau/ovs_exporter/pkg/ovs_exporter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

func main() {
	var metricsPath = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.",).Default("/metrics").String()
	var pollTimeout = kingpin.Flag("ovs.timeout", "Timeout on JSON-RPC requests to OVS.").Default("2").Int()
	var pollInterval = kingpin.Flag("ovs.poll-interval", "The minimum interval (in seconds) between collections from OVS server.").Default("15").Int()
	var isShowVersion = kingpin.Flag("version", "version information").Default("false").Bool()
	var logLevel = kingpin.Flag("log.level", "logging severity level").Default("info").String()
	var systemRunDir = kingpin.Flag("system.run.dir", "OVS default run directory.").Default("/var/run/openvswitch").String()
	var databaseVswitchName = kingpin.Flag("database.vswitch.name", "The name of OVS db.").Default("Open_vSwitch").String()
	var databaseVswitchSocketRemote = kingpin.Flag("database.vswitch.socket.remote", "JSON-RPC unix socket to OVS db.").Default("unix:/var/run/openvswitch/db.sock").String()
	var databaseVswitchFileDataPath = kingpin.Flag("database.vswitch.file.data.path", "OVS db file.").Default("/etc/openvswitch/conf.db").String()
	var databaseVswitchFileLogPath = kingpin.Flag("database.vswitch.file.log.path", "OVS db log file.").Default("/var/log/openvswitch/ovsdb-server.log").String()
	var databaseVswitchFilePidPath = kingpin.Flag("database.vswitch.file.pid.path", "OVS db process id file.").Default("/var/run/openvswitch/ovsdb-server.pid").String()
	var databaseVswitchFileSystemIDPath = kingpin.Flag("database.vswitch.file.system.id.path", "OVS system id file.").Default("/etc/openvswitch/system-id.conf").String()
	var serviceVswitchdFileLogPath = kingpin.Flag("service.vswitchd.file.log.path", "OVS vswitchd daemon log file.").Default("/var/log/openvswitch/ovs-vswitchd.log").String()
	var serviceVswitchdFilePidPath = kingpin.Flag("service.vswitchd.file.pid.path", "OVS vswitchd daemon process id file.").Default("/var/run/openvswitch/ovs-vswitchd.pid").String()
	var serviceOvnControllerFileLogPath = kingpin.Flag("service.ovncontroller.file.log.path", "OVN controller daemon log file.").Default("/var/log/openvswitch/ovn-controller.log").String()
	var serviceOvnControllerFilePidPath = kingpin.Flag("service.ovncontroller.file.pid.path", "OVN controller daemon process id file.").Default("/var/run/openvswitch/ovn-controller.pid").String()
	var toolkitFlags = webflag.AddFlags(kingpin.CommandLine, ":9475")
	kingpin.Parse()

	if *isShowVersion {
		fmt.Fprintf(os.Stdout, "%s %s", ovs.GetExporterName(), ovs.GetVersion())
		if ovs.GetRevision() != "" {
			fmt.Fprintf(os.Stdout, ", commit: %s\n", ovs.GetRevision())
		} else {
			fmt.Fprint(os.Stdout, "\n")
		}
		os.Exit(0)
	}
	logger, error := ovs_exporter.NewLogger(*logLevel)
	if error != nil {
		panic(error)
	}
	slog.SetDefault(&logger)

	slog.Info("Starting exporter",
			  "exporter", ovs.GetExporterName(),
			  "version", ovs.GetVersionInfo(),
			  "build_context", ovs.GetVersionBuildContext(),
	)

	opts := ovs.Options{
		Timeout:                   *pollTimeout,
		Logger:                    *slog.Default(),
	}

	exporter := ovs.NewExporter(opts)

	exporter.Client.System.RunDir = *systemRunDir

	exporter.Client.Database.Vswitch.Name = *databaseVswitchName
	exporter.Client.Database.Vswitch.Socket.Remote = *databaseVswitchSocketRemote
	exporter.Client.Database.Vswitch.File.Data.Path = *databaseVswitchFileDataPath
	exporter.Client.Database.Vswitch.File.Log.Path = *databaseVswitchFileLogPath
	exporter.Client.Database.Vswitch.File.Pid.Path = *databaseVswitchFilePidPath
	exporter.Client.Database.Vswitch.File.SystemID.Path = *databaseVswitchFileSystemIDPath

	exporter.Client.Service.Vswitchd.File.Log.Path = *serviceVswitchdFileLogPath
	exporter.Client.Service.Vswitchd.File.Pid.Path = *serviceVswitchdFilePidPath

	exporter.Client.Service.OvnController.File.Log.Path = *serviceOvnControllerFileLogPath
	exporter.Client.Service.OvnController.File.Pid.Path = *serviceOvnControllerFilePidPath
	if err := exporter.Connect(); err != nil {
		slog.Error("failed to init properly", "error", err.Error(),)
		os.Exit(1)
	}

	slog.Info("ovs_system_id", "ovs_system_id", exporter.Client.System.ID)

	exporter.SetPollInterval(int64(*pollInterval))
	prometheus.MustRegister(exporter)

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>OVS Exporter</title></head>
             <body>
             <h1>OVS Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})

	server := &http.Server{}
	if err := web.ListenAndServe(server, toolkitFlags, slog.Default()); err != nil {
		slog.Error("listener failed", "error", err.Error(),
		)
		os.Exit(1)
	}
}

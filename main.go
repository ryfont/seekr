package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/seekr-osint/seekr/api"
	"github.com/seekr-osint/seekr/api/config"
	"github.com/seekr-osint/seekr/api/seekrd"
	seekrdhandler "github.com/seekr-osint/seekr/api/seekrdHandler"
	"github.com/seekr-osint/seekr/api/version"

	"github.com/seekr-osint/seekr/api/discord"
	"github.com/seekr-osint/seekr/api/server"
	"github.com/seekr-osint/seekr/api/webserver"
	"github.com/seekr-osint/seekr/seekrplugin"
)

// Web server content
//
//go:embed web/*
var content embed.FS

var dataBase = make(api.DataBase)

var ver string

var schematicVersion version.SchematicVersion

func main() {
	if ver != "" {
		fmt.Printf("Welcome to seekr v%s\n", ver)
		schematicVersion, err := version.ParseSchematicVersion(ver)
		if err != nil {
			log.Panicf("error checking version: %s\n", ver)
		}
		if !schematicVersion.IsLatest() {
			fmt.Printf("You are running an old seekr version.\nDownload the latest seekr version at: %s\n", schematicVersion.GetLatest().DownloadURL())
		}

	} else {
		fmt.Printf("Welcome to seekr unstable\nplease note that this version of seekr is NOT officially supported\n")
	}

	cfg, err := config.LoadConfig()
	if err != nil && err != config.ErrNoConfigFile {
		fmt.Printf("Failed to load config: %s\n", err)
		return
	}
	configError := err
	// dir := flag.String("dir", "./web", "dir where the html source code is located")
	ip := flag.String("ip", cfg.Server.Ip, "Ip to serve api + webServer on (0.0.0.0 or localhost usually)")
	data := flag.String("db", "data", "Database location")
	port := flag.Uint64("port", cfg.Server.Port, "Port to serve the API on")
	enableWebserver := flag.Bool("webserver", true, "Enable the webserver")

	browser := flag.Bool("browser", cfg.General.Browser, "open up the html interface in the default web browser")
	forcePort := flag.Bool("forcePort", cfg.General.ForcePort, "forcePort")

	createConfig := flag.Bool("writeDefaultConfig", false, "create toml config file containing the default config if the config is invalid or doesn't exsist")

	enableRichCord := flag.Bool("discord", cfg.General.Discord, "Enable the discord rich appearance")
	//enableWebserver := flag.Bool("webserver", true, "Enable the webserver")
	enableApiServer := true
	// webserverPort := flag.String("webserverPort", "5050", "Port to serve webserver on")
	pluginList := os.Getenv("SEEKR_PLUGINS")
	plugins := []string{}
	if pluginList != "" {
		plugins = strings.Split(pluginList, ",")
	}
	flag.Parse()
	if configError == config.ErrNoConfigFile && *createConfig {
		err = config.CreateDefaultConfig()
		if err != nil {
			fmt.Printf("error: %s\n", err)
		}
	}
	if *enableRichCord {
		err := discord.Rich()
		if err == nil {
			// No error printing due it printing an error if discord is not running / installed
			//fmt.Printf("%s\n", err)
			fmt.Printf("Setting discord rich presence\n")
		}
	}
	apiConfig, err := api.ApiConfig{
		Config:  cfg,
		Version: schematicVersion,
		Server: server.Server{
			Ip:        *ip,
			Port:      uint16(*port),
			ForcePort: *forcePort,
			WebServer: webserver.Webserver{
				Disable:    !*enableWebserver,
				FileSystem: content,
			},
			ApiServer: server.ApiServer{
				Disable: !enableApiServer,
			},
		},
		LogFile:       "seekr.log",
		DataBaseFile:  *data,
		DataBase:      dataBase,
		SetCORSHeader: true,
		SaveDBFunc:    api.DefaultSaveDB,
		LoadDBFunc:    api.DefaultLoadDB,
	}.ConfigParse()
	if err != nil {
		log.Panicf("error: %s", err)
	}
	apiConfig, err = seekrplugin.Open(plugins, apiConfig)
	if err != nil {
		log.Panicf("error: %s", err)
	}

	if *browser && !apiConfig.Server.WebServer.Disable {
		openbrowser(fmt.Sprintf("http://%s:%d/web/index.html", apiConfig.Server.Ip, apiConfig.Server.Port))
	}
	//fmt.Println("Welcome to seekr a powerful OSINT tool able to scan the web for " + strconv.Itoa(len(api.DefaultServices)) + "services")
	seekrdInstance := seekrd.SeekrdInstance{
		Interval:  30,
		ApiConfig: seekrd.ApiConfig(&apiConfig),
		Services: seekrd.SeekrdServices{
			seekrd.SeekrdService{
				Name: "test",
				Func: seekrd.SeekrdFunc(seekrdhandler.Handler(func(apiConfig *api.ApiConfig) error {
					//apiConfig.DataBase["1"] = api.Person{
					//ID:   "1",
					//Name: "hacker supa hack hack hack",
					//}
					return nil
				})),
				Repeat: true,
			},
		},
	}
	go seekrdInstance.SeekrdTicker()
	//go api.Seekrd(api.DefaultSeekrdServices, 30) // run every 30 minutes
	api.ServeApi(apiConfig)
}

func openbrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("cmd", "/c", "start", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	}
	api.Check(err)
}

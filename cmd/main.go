package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws-samples/prometheus-for-ecs/pkg/aws"
)

const (
	PrometheusConfigParameter    = "ECS-Prometheus-Configuration"
	DiscoveryNamespacesParameter = "ECS-ServiceDiscovery-Namespaces"
	ScrapeConfigFile             = "ecs-services.json"
)

var allowReloadPrometheusConfig bool
var prometheusConfigFilePath string
var scrapeConfigFilePath string

func main() {
	log.Println("Prometheus configuration reloader started")
	aws.InitializeAWSSession()

	configFileDir, present := os.LookupEnv("CONFIG_FILE_DIR")
	if !present {
		configFileDir = "/etc/config/"
	}
	reloadConfig, present := os.LookupEnv("PROMETHEUS_RELOAD_CONFIG")
	if present {
		b, err := strconv.ParseBool(reloadConfig)
		if err == nil {
			allowReloadPrometheusConfig = b
			log.Println("Prometheus configuration reloader failed to parse option 'PROMETHEUS_RELOAD_CONFIG'")
		}
	}

	configReloadFrequency, present := os.LookupEnv("CONFIG_RELOAD_FREQUENCY")
	if !present {
		configReloadFrequency = "30"
	}

	scrapeConfigFile, present := os.LookupEnv("SCRAPE_CONFIG_FILE")
	if !present {
		scrapeConfigFile = ScrapeConfigFile
	}

	prometheusConfigFilePath = strings.Join([]string{configFileDir, "prometheus.yaml"}, "/")
	scrapeConfigFilePath = strings.Join([]string{configFileDir, scrapeConfigFile}, "/")

	loadPrometheusConfig()
	loadScrapeConfig()
	log.Println("Loaded initial configuration file")

	go func() {
		reloadFrequency, _ := strconv.Atoi(configReloadFrequency)
		ticker := time.NewTicker(time.Duration(reloadFrequency) * time.Second)
		reloadPrometheusConfig := false
		for {
			select {
			case <-ticker.C:
				//
				// Ticker contains a channel
				// It sends the time on the channel after the number of ticks specified by the duration have elapsed.
				//
				// we reload prometheus config a half the frequency of the scrape
				if reloadPrometheusConfig && allowReloadPrometheusConfig {
					loadPrometheusConfig()
				}
				reloadPrometheusConfig = !reloadPrometheusConfig
				reloadScrapeConfig()
			}
		}
	}()
	log.Println("Periodic reloads under progress...")

	//
	// Block indefinitely on the main channel
	//
	stopChannel := make(chan string)

	for {
		select {
		case status := <-stopChannel:
			fmt.Println(status)
			break
		}
	}
}

func loadPrometheusConfig() {
	prometheusConfigParameter, present := os.LookupEnv("PROMETHEUS_CONFIG_SSMPARAM_NAME")
	if !present {
		prometheusConfigParameter = PrometheusConfigParameter
	}
	prometheusConfig := aws.GetParameter(prometheusConfigParameter)
	err := ioutil.WriteFile(prometheusConfigFilePath, []byte(*prometheusConfig), 0644)
	if err != nil {
		log.Println(err)
	}
}

func loadScrapeConfig() {
	err := ioutil.WriteFile(scrapeConfigFilePath, []byte("[]"), 0644)
	if err != nil {
		log.Println(err)
	}
}

func reloadScrapeConfig() {
	discoveryNamespacesParameter, present := os.LookupEnv("DISCOVERY_NAMESPACE_SSMPARAM")
	if !present {
		discoveryNamespacesParameter = DiscoveryNamespacesParameter
	}
	namespaceList := aws.GetParameter(discoveryNamespacesParameter)
	namespaces := strings.Split(*namespaceList, ",")
	scrapConfig := aws.GetPrometheusScrapeConfig(namespaces)
	err := ioutil.WriteFile(scrapeConfigFilePath, []byte(*scrapConfig), 0644)
	if err != nil {
		log.Println(err)
	}
}

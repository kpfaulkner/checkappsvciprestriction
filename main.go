package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/web/mgmt/2018-02-01/web"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"log"
	"strings"
)

type configWrapper struct {

	appServiceName string
	resourceGroup string
	config *web.SiteConfigResource
}

func init() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: ./checkappsvciprestriction  <get/set> <app service prefix> <rule name> <priority> <ip>")
		os.Exit(-1)
	}

	if os.Args[1] != "get" && os.Args[1] != "set" {
		fmt.Println("Usage: ./checkappsvciprestriction  <get/set> <app service prefix> <rule name> <priority> <ip>")
		os.Exit(-1)
	}

	if os.Args[1] == "set" && len(os.Args) != 6 {
		fmt.Println("Usage: ./checkappsvciprestriction  <get/set> <app service prefix> <rule name> <priority> <ip>")
		os.Exit(-1)
	}

}

func getAppServiceWithPrefix( prefix string, client web.AppsClient ) ([]web.Site, error) {

	appServiceList := make([]web.Site, 0)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()

	apps, err := client.ListComplete(ctx)
	if err != nil {
		log.Fatalf("unable to get list of appservices", err)
	}


	for apps.NotDone() {
		v := apps.Value()
		if strings.HasPrefix(*v.Name, prefix) {
			appServiceList = append(appServiceList, v)
		}
		apps.NextWithContext(ctx)
	}

	return appServiceList, nil
}

func getSiteConfigList( appServices []web.Site, client web.AppsClient  ) ([]configWrapper, error) {

	configWrapperList := make([]configWrapper,0,5)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()

	for _, app := range appServices {
		config, err := client.ListConfigurationsComplete(ctx, *app.ResourceGroup, *app.Name)
		if err != nil {
			log.Fatalf("unable to list configs", err)
		}

		cv := config.Value()

		cw := configWrapper{}
		cw.appServiceName = *app.Name
		cw.resourceGroup = *app.ResourceGroup
		cw.config = &cv
		configWrapperList = append(configWrapperList, cw)

	}

	return configWrapperList, nil
}

// display to stdout.
func displayIPRestrictions( configWrapperList []configWrapper ) {

	for _,cw := range configWrapperList {

		if cw.config.IPSecurityRestrictions != nil {
			fmt.Printf("App service %s\n", cw.appServiceName)
			for _,ip := range *cw.config.IPSecurityRestrictions {
				fmt.Printf("IP Restriction: name %s: priority %d: ip %s\n", *ip.Name, *ip.Priority, *ip.IPAddress)
			}
		}
	}
}

func setIPRestrictions( client web.AppsClient , configList []configWrapper, ip string, priority int32, name string) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()

	for _, cfg := range configList {
		if cfg.config.IPSecurityRestrictions != nil {
			for _, i := range *cfg.config.IPSecurityRestrictions {
				if *i.Name == name {
					*i.IPAddress = ip
					*i.Priority = priority

					_, err := client.UpdateConfiguration(ctx, cfg.resourceGroup, cfg.appServiceName, *cfg.config)
					if err != nil {
						fmt.Printf("error updating %v\n", err)
					}
					fmt.Printf("updated... I hope\n")
				}
			}

		}
	}
}

func main() {

	operation := os.Args[1]

	// details we need to modify.
	subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")

	a, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		log.Fatalf("Unable to create initialiser!!! %v\n", err)
	}

	client := web.NewAppsClient(subscriptionID)
	client.Authorizer = a

	appServiceList,err := getAppServiceWithPrefix( "kenfautest", client)
	if err != nil {
		log.Fatalf("unable to get list of appservices", err)
	}

	siteConfigList, err := getSiteConfigList( appServiceList, client)
	if err != nil {
		log.Fatalf("unable to get list of configs", err)
	}

	if operation == "get" {
		displayIPRestrictions( siteConfigList)
	} else if operation == "set" {
		ip := os.Args[5]
		priorityInt,_ := strconv.Atoi( os.Args[4])
		priority := int32( priorityInt)
		ruleName := os.Args[3]
		setIPRestrictions(client, siteConfigList, ip, priority, ruleName )
	}
}

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/ReSTARTR/ec2-ls-hosts/client"
	"github.com/ReSTARTR/ec2-ls-hosts/creds"
	"gopkg.in/ini.v1"
)

var (
	version string
)

func loadRegionInAwsConfig(profile string) string {
	cfg, err := creds.LoadAwsConfig()
	if err == nil {
		return cfg.Section(profile).Key("region").Value()
	}
	return ""
}

func loadConfig() (cfg *ini.File, err error) {
	cfg, err = ini.LooseLoad(
		os.Getenv("HOME")+"/.ls-hosts",
		"/etc/ls-hosts.conf",
	)
	if err != nil {
		return
	}
	return
}

// string to map[string]string
func parseFilterString(s string) map[string]string {
	filters := make(map[string]string, 5)
	for _, kv := range strings.Split(s, ",") {
		a := strings.Split(kv, ":")
		if len(a) > 1 {
			v := a[1:len(a)]
			filters[a[0]] = strings.Join(v, ":")
		}
	}
	return filters
}

// string to []string
func parseFieldsString(str string) []string {
	var fields []string
	for _, c := range strings.Split(str, ",") {
		fields = append(fields, c)
	}
	return fields
}

func optionsFromFile() *client.Options {
	opt := client.NewOptions()
	if cfg, err := loadConfig(); err == nil {
		opt.Profile = cfg.Section("options").Key("profile").Value()
		opt.Region = cfg.Section("options").Key("region").Value()
		opt.TagFilters = parseFilterString(cfg.Section("options").Key("tags").Value())
		opt.Fields = parseFieldsString(cfg.Section("options").Key("fields").Value())
		opt.Credentials = cfg.Section("options").Key("creds").Value()
		opt.Noheader = (cfg.Section("options").Key("noheader").Value() == "true")
	}
	return opt
}

func NewTableWriter() *tabwriter.Writer {
	table := new(tabwriter.Writer)
	table.Init(os.Stdout, 1, 8, 1, '\t', 0)
	return table
}

func main() {
	// parse options
	profile := flag.String("profile", "default", "profile name of aws credentials")
	filters := flag.String("filters", "", "key1:value1,key2:value2,...")
	tagFilters := flag.String("tags", "", "key1:value1,key2:value2,...")
	fields := flag.String("fields", "", "column1,column2,...")
	regionString := flag.String("region", "", "region name")
	credsString := flag.String("creds", "", "env, shared, iam")
	noheader := flag.Bool("noheader", false, "hide header")
	v := flag.Bool("v", false, "show version")
	flag.Parse()
	if *v {
		fmt.Println("version: " + version)
		os.Exit(0)
	}

	opt := optionsFromFile()
	opt.Profile = *profile
	awsConfigRegion := loadRegionInAwsConfig(opt.Profile)
	if awsConfigRegion != "" {
		opt.Region = awsConfigRegion
	}
	// merge optoins from cmdline
	if *filters != "" {
		for k, v := range parseFilterString(*filters) {
			opt.Filters[k] = v
		}
	}
	if *tagFilters != "" {
		for k, v := range parseFilterString(*tagFilters) {
			opt.TagFilters[k] = v
		}
	}
	if *fields != "" {
		opt.Fields = parseFieldsString(*fields)
	}
	if *regionString != "" {
		opt.Region = *regionString
	}
	if *credsString != "" {
		opt.Credentials = *credsString
	}
	opt.Noheader = opt.Noheader || *noheader

	// run
	w := NewTableWriter()
	err := client.Describe(opt, w)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
}

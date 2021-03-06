package client

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/ReSTARTR/ec2-ls-hosts/creds"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type Writer interface {
	SetHeader(s []string)
	Append(s []string)
	Render()
}

var (
	defaultFields = []string{
		"tag:Name",
		"instance-id",
		"private-ip",
		"public-ip",
		"instance-state",
	}
)

type Options struct {
	Profile     string
	Filters     map[string]string
	TagFilters  map[string]string
	Fields      []string
	Region      string
	Credentials string
	Noheader    bool
}

func NewOptions() *Options {
	opt := &Options{}
	opt.Filters = make(map[string]string)
	opt.TagFilters = make(map[string]string)
	return opt
}

func (o *Options) FieldNames() []string {
	if len(o.Fields) > 1 {
		return o.Fields
	}
	return defaultFields
}

func namesToUpper(strs []string) []string {
	ss := make([]string, len(strs))
	for i, s := range strs {
		ss[i] = strings.ToUpper(s)
	}
	return ss
}

func Describe(o *Options, w *tabwriter.Writer) error {
	// build queries
	config := &aws.Config{Region: aws.String(o.Region)}
	credentials, err := creds.SelectCredentials(o.Credentials, o.Profile)
	if err != nil {
		return err
	}
	config.Credentials = credentials
	svc := ec2.New(session.New(), config)

	// call aws api
	options := &ec2.DescribeInstancesInput{}
	for k, v := range o.Filters {
		options.Filters = append(options.Filters, &ec2.Filter{
			Name:   aws.String(k),
			Values: []*string{aws.String(v)},
		})
	}
	for k, v := range o.TagFilters {
		options.Filters = append(options.Filters, &ec2.Filter{
			Name:   aws.String("tag:" + k),
			Values: []*string{aws.String(v)},
		})
	}

	// show info
	resp, err := svc.DescribeInstances(options)
	if err != nil {
		return err
	}
	if len(resp.Reservations) == 0 {
		return errors.New("Not Found")
	}

	if o.Noheader == false {
		names := namesToUpper(o.FieldNames())
		fmt.Fprintln(w, strings.Join(names, "\t"))
	}
	for idx, _ := range resp.Reservations {
		for _, inst := range resp.Reservations[idx].Instances {
			values := formatInstance(inst, o.FieldNames())
			fmt.Fprintln(w, strings.Join(values, "\t"))
		}
	}
	w.Flush()

	return nil
}

func formatInstance(inst *ec2.Instance, fields []string) []string {
	// fetch IPs
	var privateIps []string
	var publicIps []string
	for _, nic := range inst.NetworkInterfaces {
		for _, privateIp := range nic.PrivateIpAddresses {
			privateIps = append(privateIps, *privateIp.PrivateIpAddress)
			if privateIp.Association != nil {
				publicIps = append(publicIps, *privateIp.Association.PublicIp)
				break
			}
		}
	}

	// fetch tags
	// NOTE: *DO NOT* support multiple tag values
	tags := make(map[string]string, 5)
	for _, tag := range inst.Tags {
		tags[*tag.Key] = *tag.Value
	}

	var values []string
	for _, c := range fields {
		switch c {
		case "instance-id":
			values = append(values, *inst.InstanceId)
		case "private-ip":
			values = append(values, strings.Join(privateIps, ","))
		case "public-ip":
			values = append(values, strings.Join(publicIps, ","))
		case "launch-time":
			values = append(values, inst.LaunchTime.String())
		case "instance-state":
			values = append(values, *inst.State.Name)
		default:
			// extract key-values as tag string
			matched, err := regexp.Match("tag:.+", []byte(c))
			if err == nil && matched {
				kv := strings.Split(c, ":")
				key := strings.Join(kv[1:len(kv)], ":")
				if v, ok := tags[key]; ok {
					values = append(values, v)
				}
			}
		}
	}

	return values
}

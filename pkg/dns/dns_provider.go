/*
Copyright 2022 The MultiCluster Traffic Controller Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dns

import (
	"fmt"

	dnsAWS "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/dns/aws"
)

func DNSProvider(dnsProviderName string) (Provider, error) {
	var dnsProvider Provider
	var dnsError error
	switch dnsProviderName {
	case "aws":
		dnsProvider, dnsError = newAWSDNSProvider()
	default:
		dnsProvider = &FakeProvider{}
	}
	return dnsProvider, dnsError
}

func newAWSDNSProvider() (Provider, error) {
	var dnsProvider Provider
	provider, err := dnsAWS.NewProvider(dnsAWS.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS DNS manager: %v", err)
	}
	dnsProvider = provider

	return dnsProvider, nil
}

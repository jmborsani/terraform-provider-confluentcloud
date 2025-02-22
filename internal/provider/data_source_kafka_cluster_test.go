// Copyright 2021 Confluent Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"context"
	"fmt"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/walkerus/go-wiremock"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

const (
	dataSourceKafkaScenarioName = "confluentcloud_kafka Data Source Lifecycle"
)

var fullKafkaDataSourceLabel = fmt.Sprintf("data.confluentcloud_kafka_cluster.%s", kafkaResourceLabel)

func TestAccDataSourceCluster(t *testing.T) {
	containerPort := "8080"
	containerPortTcp := fmt.Sprintf("%s/tcp", containerPort)
	ctx := context.Background()
	listeningPort := wait.ForListeningPort(nat.Port(containerPortTcp))
	req := testcontainers.ContainerRequest{
		Image:        "rodolpheche/wiremock",
		ExposedPorts: []string{containerPortTcp},
		WaitingFor:   listeningPort,
	}
	wiremockContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	require.NoError(t, err)

	// nolint:errcheck
	defer wiremockContainer.Terminate(ctx)

	host, err := wiremockContainer.Host(ctx)
	require.NoError(t, err)

	wiremockHttpMappedPort, err := wiremockContainer.MappedPort(ctx, nat.Port(containerPort))
	require.NoError(t, err)

	mockServerUrl := fmt.Sprintf("http://%s:%s", host, wiremockHttpMappedPort.Port())
	wiremockClient := wiremock.NewClient(mockServerUrl)
	// nolint:errcheck
	defer wiremockClient.Reset()

	// nolint:errcheck
	defer wiremockClient.ResetAllScenarios()

	readCreatedClusterResponse, _ := ioutil.ReadFile("../testdata/kafka/read_created_kafka.json")
	_ = wiremockClient.StubFor(wiremock.Get(wiremock.URLPathEqualTo(readKafkaPath)).
		InScenario(dataSourceKafkaScenarioName).
		WithQueryParam("environment", wiremock.EqualTo(kafkaEnvId)).
		WhenScenarioStateIs(wiremock.ScenarioStateStarted).
		WillReturn(
			string(readCreatedClusterResponse),
			contentTypeJSONHeader,
			http.StatusOK,
		))

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		// https://www.terraform.io/docs/extend/testing/acceptance-tests/teststep.html
		// https://www.terraform.io/docs/extend/best-practices/testing.html#built-in-patterns
		Steps: []resource.TestStep{
			{
				Config: testAccCheckDataSourceClusterConfig(mockServerUrl, paramBasicCluster),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckClusterExists(fullKafkaDataSourceLabel),
					resource.TestCheckResourceAttr(fullKafkaDataSourceLabel, "id", kafkaClusterId),
					resource.TestCheckResourceAttr(fullKafkaDataSourceLabel, "availability", kafkaAvailability),
					resource.TestCheckResourceAttr(fullKafkaDataSourceLabel, "bootstrap_endpoint", kafkaBootstrapEndpoint),
					resource.TestCheckResourceAttr(fullKafkaDataSourceLabel, "cloud", kafkaCloud),
					resource.TestCheckResourceAttr(fullKafkaDataSourceLabel, "basic.#", "1"),
					resource.TestCheckResourceAttr(fullKafkaDataSourceLabel, "basic.0.%", "0"),
					resource.TestCheckResourceAttr(fullKafkaDataSourceLabel, "standard.#", "0"),
					resource.TestCheckResourceAttr(fullKafkaDataSourceLabel, "environment.#", "1"),
					resource.TestCheckResourceAttr(fullKafkaDataSourceLabel, "environment.0.id", kafkaEnvId),
					resource.TestCheckResourceAttr(fullKafkaDataSourceLabel, "http_endpoint", kafkaHttpEndpoint),
					resource.TestCheckResourceAttr(fullKafkaDataSourceLabel, "rbac_crn", kafkaRbacCrn),
				),
			},
		},
	})
}

func testAccCheckDataSourceClusterConfig(mockServerUrl, clusterType string) string {
	return fmt.Sprintf(`
	provider "confluentcloud" {
 		endpoint = "%s"
	}
	data "confluentcloud_kafka_cluster" "basic-cluster" {
		id = "%s"
	  	environment {
			id = "%s"
	  	}
	}
	`, mockServerUrl, kafkaClusterId, kafkaEnvId)
}

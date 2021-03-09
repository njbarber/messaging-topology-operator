/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package internal

import (
	"encoding/json"
	"fmt"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
)

func GenerateExchangeSettings(e *topologyv1alpha1.Exchange) (*rabbithole.ExchangeSettings, error) {
	arguments := make(map[string]interface{})
	if e.Spec.Arguments != nil {
		if err := json.Unmarshal(e.Spec.Arguments.Raw, &arguments); err != nil {
			return nil, fmt.Errorf("failed to unmarshall exchange arguments: %v", err)
		}
	}

	return &rabbithole.ExchangeSettings{
		Durable:    e.Spec.Durable,
		AutoDelete: e.Spec.AutoDelete,
		Type:       e.Spec.Type,
		Arguments:  arguments,
	}, nil
}
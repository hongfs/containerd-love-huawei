/*
   Copyright The containerd Authors.

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

package sbserver

import (
	"context"
	"encoding/json"
	"fmt"
	goruntime "runtime"

	"github.com/containerd/log"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// networkNotReadyReason is the reason reported when network is not ready.
const networkNotReadyReason = "NetworkPluginNotReady"

// Status returns the status of the runtime.
func (c *criService) Status(ctx context.Context, r *runtime.StatusRequest) (*runtime.StatusResponse, error) {
	// As a containerd plugin, if CRI plugin is serving request,
	// containerd must be ready.
	runtimeCondition := &runtime.RuntimeCondition{
		Type:   runtime.RuntimeReady,
		Status: true,
	}
	networkCondition := &runtime.RuntimeCondition{
		Type:   runtime.NetworkReady,
		Status: true,
	}
	netPlugin := c.netPlugin[defaultNetworkPlugin]
	// Check the status of the cni initialization
	if netPlugin != nil {
		if err := netPlugin.Status(); err != nil {
			networkCondition.Status = false
			networkCondition.Reason = networkNotReadyReason
			networkCondition.Message = fmt.Sprintf("Network plugin returns error: %v", err)
		}
	}

	resp := &runtime.StatusResponse{
		Status: &runtime.RuntimeStatus{Conditions: []*runtime.RuntimeCondition{
			runtimeCondition,
			networkCondition,
		}},
	}
	if r.Verbose {
		configByt, err := json.Marshal(c.config)
		if err != nil {
			return nil, err
		}
		resp.Info = make(map[string]string)
		resp.Info["config"] = string(configByt)
		versionByt, err := json.Marshal(goruntime.Version())
		if err != nil {
			return nil, err
		}
		resp.Info["golang"] = string(versionByt)

		if netPlugin != nil {
			cniConfig, err := json.Marshal(netPlugin.GetConfig())
			if err != nil {
				log.G(ctx).WithError(err).Errorf("Failed to marshal CNI config %v", err)
			}
			resp.Info["cniconfig"] = string(cniConfig)
		}

		defaultStatus := "OK"
		for name, h := range c.cniNetConfMonitor {
			s := "OK"
			if h == nil {
				continue
			}
			if lerr := h.lastStatus(); lerr != nil {
				s = lerr.Error()
			}
			resp.Info[fmt.Sprintf("lastCNILoadStatus.%s", name)] = s
			if name == defaultNetworkPlugin {
				defaultStatus = s
			}
		}
		resp.Info["lastCNILoadStatus"] = defaultStatus
	}
	return resp, nil
}

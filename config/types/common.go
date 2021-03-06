// Copyright 2017 CoreOS, Inc.
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

package types

import (
	"fmt"
	"reflect"

	ignTypes "github.com/coreos/ignition/config/v2_0/types"
	"github.com/coreos/ignition/config/validate/report"

	"github.com/coreos/container-linux-config-transpiler/config/templating"
	"github.com/coreos/container-linux-config-transpiler/config/types/util"
)

var (
	ErrPlatformUnspecified = fmt.Errorf("platform must be specified to use templating")
)

func init() {
	register2_0(func(in Config, out ignTypes.Config, platform string) (ignTypes.Config, report.Report) {
		if platform == templating.PlatformOpenStackMetadata {
			out.Systemd.Units = append(out.Systemd.Units, ignTypes.SystemdUnit{
				Name: "coreos-metadata.service",
				DropIns: []ignTypes.SystemdUnitDropIn{{
					Name:     "20-clct-provider-override.conf",
					Contents: fmt.Sprintf("[Service]\nEnvironment=COREOS_METADATA_OPT_PROVIDER=--provider=%s", platform),
				}},
			})
		}
		return out, report.Report{}
	})
}

func isZero(v interface{}) bool {
	if v == nil {
		return true
	}
	zv := reflect.Zero(reflect.TypeOf(v))
	return reflect.DeepEqual(v, zv.Interface())
}

// assembleUnit will assemble the contents of a systemd unit dropin that will
// have the given environment variables, and call the given exec line with the
// provided args prepended to it
func assembleUnit(exec string, args, vars []string, platform string) (util.SystemdUnit, error) {
	hasTemplating := templating.HasTemplating(args)

	out := util.NewSystemdUnit()
	if hasTemplating {
		if platform == "" {
			return util.SystemdUnit{}, ErrPlatformUnspecified
		}
		out.Unit.Add("Requires=coreos-metadata.service")
		out.Unit.Add("After=coreos-metadata.service")
		out.Service.Add("EnvironmentFile=/run/metadata/coreos")
		var err error
		args, err = templating.PerformTemplating(platform, args)
		if err != nil {
			return util.SystemdUnit{}, err
		}
	}

	for _, v := range vars {
		out.Service.Add(fmt.Sprintf("Environment=\"%s\"", v))
	}
	for _, a := range args {
		exec += fmt.Sprintf(" \\\n  %s", a)
	}
	out.Service.Add("ExecStart=")
	out.Service.Add(fmt.Sprintf("ExecStart=%s", exec))
	return out, nil
}

func getArgs(format, tagName string, e interface{}) []string {
	if e == nil {
		return nil
	}
	et := reflect.TypeOf(e)
	ev := reflect.ValueOf(e)

	vars := []string{}
	for i := 0; i < et.NumField(); i++ {
		if val := ev.Field(i).Interface(); !isZero(val) {
			if et.Field(i).Anonymous {
				vars = append(vars, getCliArgs(val)...)
			} else {
				key := et.Field(i).Tag.Get(tagName)
				if _, ok := val.(string); ok {
					// to handle whitespace characters
					val = fmt.Sprintf("%q", val)
				}
				vars = append(vars, fmt.Sprintf(format, key, val))
			}
		}
	}

	return vars
}

// getCliArgs builds a list of --ARG=VAL from a struct with cli: tags on its members.
func getCliArgs(e interface{}) []string {
	return getArgs("--%s=%v", "cli", e)
}

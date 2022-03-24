// Copyright 2020-2022 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bufpushaction

import (
	"github.com/bufbuild/buf/private/pkg/app"
	"github.com/bufbuild/buf/private/pkg/app/appflag"
	"github.com/bufbuild/buf/private/pkg/app/applog"
	"github.com/bufbuild/buf/private/pkg/app/appname"
	"github.com/bufbuild/buf/private/pkg/app/appverbose"
)

// appflagContainer implements appflag.Container.
type appflagContainer struct {
	app.EnvContainer
	app.StdinContainer
	app.StdoutContainer
	app.StderrContainer
	app.ArgContainer
	appnameContainer
	applogContainer
	appverboseContainer
}

func newContainerWithEnvOverrides(container appflag.Container, env map[string]string) appflag.Container {
	return &appflagContainer{
		EnvContainer:        app.NewEnvContainerWithOverrides(container, env),
		StdinContainer:      container,
		StdoutContainer:     container,
		StderrContainer:     container,
		ArgContainer:        container,
		appnameContainer:    container,
		applogContainer:     container,
		appverboseContainer: container,
	}
}

type appnameContainer interface {
	appname.Container
}

type applogContainer interface {
	applog.Container
}

type appverboseContainer interface {
	appverbose.Container
}

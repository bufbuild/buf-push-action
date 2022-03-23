package bufpushaction

import (
	"github.com/bufbuild/buf/private/pkg/app"
	"github.com/bufbuild/buf/private/pkg/app/appflag"
	"github.com/bufbuild/buf/private/pkg/app/applog"
	"github.com/bufbuild/buf/private/pkg/app/appname"
	"github.com/bufbuild/buf/private/pkg/app/appverbose"
	"github.com/bufbuild/buf/private/pkg/verbose"
	"go.uber.org/zap"
)

// appflagContainer implements appflag.Container.
type appflagContainer struct {
	app.EnvContainer
	app.StdinContainer
	app.StdoutContainer
	app.StderrContainer
	app.ArgContainer
	nameContainer    appname.Container
	logContainer     applog.Container
	verboseContainer appverbose.Container
}

func (c *appflagContainer) AppName() string {
	return c.nameContainer.AppName()
}

func (c *appflagContainer) ConfigDirPath() string {
	return c.nameContainer.ConfigDirPath()
}

func (c *appflagContainer) CacheDirPath() string {
	return c.nameContainer.CacheDirPath()
}

func (c *appflagContainer) DataDirPath() string {
	return c.nameContainer.DataDirPath()
}

func (c *appflagContainer) Port() (uint16, error) {
	return c.nameContainer.Port()
}

func (c *appflagContainer) Logger() *zap.Logger {
	return c.logContainer.Logger()
}

func (c *appflagContainer) VerbosePrinter() verbose.Printer {
	return c.verboseContainer.VerbosePrinter()
}

func newContainerWithEnvOverrides(container appflag.Container, env map[string]string) appflag.Container {
	return &appflagContainer{
		EnvContainer:     app.NewEnvContainerWithOverrides(container, env),
		StdinContainer:   container,
		StdoutContainer:  container,
		StderrContainer:  container,
		ArgContainer:     container,
		nameContainer:    container,
		logContainer:     container,
		verboseContainer: container,
	}
}

package confx

import (
	"bytes"
	"fmt"
	"path/filepath"
)

type DockerConfig struct {
	BuildImage   string
	RuntimeImage string
	GoProxy      GoProxyConfig
	Openapi      bool
}

type GoProxyConfig struct {
	ProxyOn bool
	Host    string
}

func (c *DockerConfig) setDefaults() {
	if c.BuildImage == "" {
		c.BuildImage = "dockerproxy.com/library/golang:1.20-buster"
	}
	if c.RuntimeImage == "" {
		c.RuntimeImage = "gcr.dockerproxy.com/distroless/static-debian11"
	}
	if c.GoProxy.ProxyOn {
		if c.GoProxy.Host == "" {
			c.GoProxy.Host = "https://goproxy.cn,direct"
		}
	}
}

func (c *Configuration) dockerfile() []byte {
	c.dockerConfig.setDefaults()
	dockerfile := bytes.NewBuffer(nil)
	// builder
	_, _ = fmt.Fprintf(dockerfile, "FROM %s AS build-env\n", c.dockerConfig.BuildImage)

	_, _ = fmt.Fprintln(dockerfile, `
FROM build-env AS builder
`)
	// go proxy
	if c.dockerConfig.GoProxy.ProxyOn {
		_, _ = fmt.Fprintln(dockerfile, fmt.Sprintf(`
ARG GOPROXY=%s`, c.dockerConfig.GoProxy.Host))
	}

	_, _ = fmt.Fprintln(dockerfile, `
WORKDIR /go/src
COPY ./ ./

# build
RUN make build WORKSPACE=`+c.WorkSpace())

	// runtime
	_, _ = fmt.Fprintln(dockerfile, fmt.Sprintf(
		`
# runtime
FROM %s`, c.dockerConfig.RuntimeImage))
	_, _ = fmt.Fprintln(dockerfile, `
COPY --from=builder `+filepath.Join("/go/src/cmd", c.WorkSpace(), c.WorkSpace())+` `+filepath.Join(`/go/bin`, c.Command.Use)+`
`)
	if c.dockerConfig.Openapi {
		_, _ = fmt.Fprintf(dockerfile,
			`COPY --from=builder %s %s
		`, filepath.Join("/go/src/cmd", c.WorkSpace(), "openapi.json"), filepath.Join("/go/bin/cmd", c.WorkSpace(), "openapi.json"))
	}

	for _, envVar := range c.defaultEnvVars.Values {
		if envVar.Value != "" {
			if envVar.IsExpose {
				_, _ = fmt.Fprintln(dockerfile, `
EXPOSE`, envVar.Value)
			}
		}
	}

	fmt.Fprintf(dockerfile, `
ARG PROJECT_NAME
ARG PROJECT_VERSION
ENV PROJECT_NAME=${PROJECT_NAME} PROJECT_VERSION=${PROJECT_VERSION}

WORKDIR /go/bin
ENTRYPOINT ["`+filepath.Join(`/go/bin`, c.Command.Use)+`"]
`)

	return dockerfile.Bytes()
}

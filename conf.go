package confx

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/go-courier/envconf"
	"github.com/go-courier/reflectx"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var Config = &Configuration{}
var RootPath = ""

func init() {
	Config.Initialize()
}

func SetConfX(serviceName string, rootDir string, dockerConfig ...DockerConfig) {
	RootPath = fmt.Sprintf("/%s", serviceName)
	if len(dockerConfig) > 0 {
		Config.dockerConfig = dockerConfig[0]
	}

	Config.Command.Use = serviceName
	_, filename, _, _ := runtime.Caller(1)
	Config.projectRoot = filepath.Join(filepath.Dir(filename), rootDir)
}

func ConfP(c interface{}) {
	tpe := reflect.TypeOf(c)
	if tpe.Kind() != reflect.Ptr {
		panic(fmt.Errorf("ConfP pass ptr for setting value"))
	}

	os.Setenv("PROJECT_NAME", Config.ProjectName())

	Config.mustScan(c)
	Config.mustMarshal(c)

	Config.log(c)

	triggerInitials(c)
}

func triggerInitials(c interface{}) {
	rv := reflectx.Indirect(reflect.ValueOf(c))
	for i := 0; i < rv.NumField(); i++ {
		value := rv.Field(i)
		if conf, ok := value.Interface().(interface{ Init() }); ok {
			conf.Init()
		}
	}
}

type Configuration struct {
	*cobra.Command
	Feature              string
	outputDir            string
	projectRoot          string
	ShouldGenerateConfig bool
	defaultEnvVars       envconf.EnvVars
	//BuildImage           string

	dockerConfig DockerConfig
}

func (conf *Configuration) ProjectName() string {
	if conf.Feature != "" {
		return conf.ServiceName() + "--" + conf.Feature
	}
	return conf.ServiceName()
}

func (conf *Configuration) WorkSpace() string {
	paths := strings.Split(conf.projectRoot, "/")
	return paths[len(paths)-1]
}

func (conf *Configuration) ServiceName() string {
	return conf.Use
}

func (conf *Configuration) Prefix() string {
	return strings.ToUpper(strings.Replace(conf.Use, "-", "_", -1))
}

func (conf *Configuration) Initialize() {
	if projectFeature, exists := os.LookupEnv("PROJECT_FEATURE"); exists {
		conf.Feature = projectFeature
	}

	conf.Command = &cobra.Command{
		PreRun: func(cmd *cobra.Command, args []string) {
			if conf.ShouldGenerateConfig {
				conf.dockerize()
			}
		},
		Run: func(cmd *cobra.Command, args []string) {

		},
	}

	if conf.Use == "" {
		conf.Use = "srv-x"
	}

	conf.PersistentFlags().
		BoolVarP(&conf.ShouldGenerateConfig, "output-docker-Config", "c", true, "output configuration of docker")
}

func (conf *Configuration) mustScan(c interface{}) {
	if err := envconf.NewDotEnvDecoder(&conf.defaultEnvVars).Decode(c); err != nil {
		panic(err)
	}
	if _, err := envconf.NewDotEnvEncoder(&conf.defaultEnvVars).Encode(c); err != nil {
		panic(err)
	}
	conf.mayMarshalFromLocal(c)
}

func (conf *Configuration) log(c interface{}) {
	envVars := envconf.NewEnvVars(conf.Prefix())
	if _, err := envconf.NewDotEnvEncoder(envVars).Encode(c); err != nil {
		panic(err)
	}
	fmt.Printf("%s", string(envVars.MaskBytes()))
}

func (conf *Configuration) mayMarshalFromLocal(c interface{}) {
	contents, err := os.ReadFile(filepath.Join(conf.projectRoot, "./config/local.yml"))
	if err != nil {
		return
	}
	keyValues := map[string]string{}
	err = yaml.Unmarshal(contents, &keyValues)
	if err != nil {
		return
	}

	envVars := &envconf.EnvVars{
		Prefix: conf.Prefix(),
	}
	for key, value := range keyValues {
		envVars.SetKeyValue(key, value)
	}
	if err := envconf.NewDotEnvDecoder(envVars).Decode(c); err != nil {
		panic(err)
	}
}

func (conf *Configuration) mustMarshal(c interface{}) {
	envVars := envconf.EnvVarsFromEnviron(conf.Prefix(), os.Environ())
	if err := envconf.NewDotEnvDecoder(envVars).Decode(c); err != nil {
		panic(err)
	}
}

func AddCommand(cmds ...*cobra.Command) {
	Config.Command.AddCommand(cmds...)
}

func Execute(run func(cmd *cobra.Command, args []string)) {
	Config.Command.Run = run

	if err := Config.Execute(); err != nil {
		panic(err)
	}
}

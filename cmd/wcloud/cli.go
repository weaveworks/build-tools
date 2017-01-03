package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"gopkg.in/yaml.v2"
)

// ArrayFlags allows you to collect repeated flags
type ArrayFlags []string

func (a *ArrayFlags) String() string {
	return strings.Join(*a, ",")
}

// Set implements flags.Value
func (a *ArrayFlags) Set(value string) error {
	*a = append(*a, value)
	return nil
}

func env(key, def string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return def
}

func contextsFile() string {
	u, err := user.Current()
	if err != nil {
		panic(err)
	}

	return filepath.Join(u.HomeDir, ".wcloudconfig")
}

func usage() {
	fmt.Printf(`Usage: wcloud COMMAND ...
	deploy <image>:<version>   Deploy image to your configured env
	list                       List recent deployments
	config (<filename>)        Get (or set) the configured env
	logs <deploy>              Show lots for the given deployment
	context (<name>)           Get (or set) the configured context

	Environment Variables:
	  SERVICE_TOKEN            Set the service token to use, overrides %s
	  BASE_URL                 Set the deploy to connect to, overrides %s
`,
		contextsFile(),
		contextsFile(),
	)
}

func main() {
	if len(os.Args) <= 1 {
		usage()
		os.Exit(1)
	}

	// We don't need to create a client for this.
	if os.Args[1] == "context" {
		context(os.Args[2:])
		return
	}

	currentContext, err := loadCurrentContext()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	token := env("SERVICE_TOKEN", currentContext.ServiceToken)
	baseURL := env("BASE_URL", currentContext.BaseURL)
	if baseURL == "" {
		baseURL = "https://cloud.weave.works"
	}

	c := NewClient(token, baseURL)

	switch os.Args[1] {
	case "deploy":
		deploy(c, os.Args[2:])
	case "list":
		list(c, os.Args[2:])
	case "config":
		config(c, os.Args[2:])
	case "logs":
		logs(c, os.Args[2:])
	case "events":
		events(c, os.Args[2:])
	case "help":
		usage()
	default:
		usage()
	}
}

func newFlagSet() *flag.FlagSet {
	flags := flag.NewFlagSet("", flag.ContinueOnError)
	flags.Usage = usage
	return flags
}

func deploy(c Client, args []string) {
	var (
		flags    = newFlagSet()
		username = flags.String("u", "", "Username to report to deploy service (default with be current user)")
		services ArrayFlags
	)
	flags.Var(&services, "service", "Service to update (can be repeated)")
	if err := flags.Parse(args); err != nil {
		usage()
		return
	}
	args = flags.Args()
	if len(args) != 1 {
		usage()
		return
	}
	parts := strings.SplitN(args[0], ":", 2)
	if len(parts) < 2 {
		usage()
		return
	}
	if *username == "" {
		user, err := user.Current()
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		*username = user.Username
	}
	deployment := Deployment{
		ImageName:        parts[0],
		Version:          parts[1],
		TriggeringUser:   *username,
		IntendedServices: services,
	}
	if err := c.Deploy(deployment); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func list(c Client, args []string) {
	var (
		flags = newFlagSet()
		since = flags.Duration("since", 7*24*time.Hour, "How far back to fetch results")
	)
	if err := flags.Parse(args); err != nil {
		usage()
		return
	}
	through := time.Now()
	from := through.Add(-*since)
	deployments, err := c.GetDeployments(from.Unix(), through.Unix())
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Created", "ID", "Image", "Version", "State"})
	table.SetBorder(false)
	table.SetColumnSeparator(" ")
	for _, deployment := range deployments {
		table.Append([]string{
			deployment.CreatedAt.Format(time.RFC822),
			deployment.ID,
			deployment.ImageName,
			deployment.Version,
			deployment.State,
		})
	}
	table.Render()
}

func events(c Client, args []string) {
	var (
		flags = newFlagSet()
		since = flags.Duration("since", 7*24*time.Hour, "How far back to fetch results")
	)
	if err := flags.Parse(args); err != nil {
		usage()
		return
	}
	through := time.Now()
	from := through.Add(-*since)
	events, err := c.GetEvents(from.Unix(), through.Unix())
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	fmt.Println("events: ", string(events))
}

func loadConfig(filename string) (*Config, error) {
	extension := filepath.Ext(filename)
	var config Config
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	if extension == ".yaml" || extension == ".yml" {
		if err := yaml.Unmarshal(buf, &config); err != nil {
			return nil, err
		}
	} else {
		if err := json.NewDecoder(bytes.NewReader(buf)).Decode(&config); err != nil {
			return nil, err
		}
	}
	return &config, err
}

func config(c Client, args []string) {
	if len(args) > 1 {
		usage()
		return
	}

	if len(args) == 1 {
		config, err := loadConfig(args[0])
		if err != nil {
			fmt.Println("Error reading config:", err)
			os.Exit(1)
		}

		if err := c.SetConfig(config); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
	} else {
		config, err := c.GetConfig()
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		buf, err := yaml.Marshal(config)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		fmt.Println(string(buf))
	}
}

func loadContexts() (ContextsFile, error) {
	filename := contextsFile()
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return ContextsFile{}, nil
		}
		return ContextsFile{}, err
	}
	var contexts ContextsFile
	if err := yaml.Unmarshal(buf, &contexts); err != nil {
		return ContextsFile{}, err
	}
	return contexts, err
}

func loadCurrentContext() (Context, error) {
	contexts, err := loadContexts()
	if err != nil {
		return Context{}, err
	}
	return contexts.Contexts[contexts.Current], nil
}

func saveContexts(i ContextsFile) error {
	buf, err := yaml.Marshal(i)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(contextsFile(), buf, 0600)
}

func context(args []string) {
	if len(args) > 1 {
		usage()
		return
	}

	if len(args) == 1 {
		// Setting the current context name
		contexts, err := loadContexts()
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		_, ok := contexts.Contexts[args[0]]
		if !ok {
			fmt.Printf("Context %q not found in %s\n", args[0], contextsFile())
			fmt.Printf("Available: %s\n", strings.Join(contexts.Available(), " "))
			os.Exit(1)
		}

		contexts.Current = args[0]
		if err := saveContexts(contexts); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
	} else {
		// Getting the current context
		contexts, err := loadContexts()
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		if contexts.Current == "" {
			fmt.Println("<none>")
			fmt.Printf("Available: %s\n", strings.Join(contexts.Available(), " "))
			os.Exit(0)
		}

		current, ok := contexts.Contexts[contexts.Current]
		if !ok {
			fmt.Printf("Context %q not found in %s\n", args[0], contextsFile())
			os.Exit(1)
		}

		buf, err := yaml.Marshal(map[string]Context{
			contexts.Current: current,
		})
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		fmt.Println(string(buf))
	}
}

func logs(c Client, args []string) {
	if len(args) != 1 {
		usage()
		return
	}

	output, err := c.GetLogs(args[0])
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	fmt.Println(string(output))
}

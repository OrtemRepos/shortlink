package configs

import (
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Repository struct {
		InMemory bool   `yaml:"inMemory" env:"IN_MEMORY" env-description:"In-memory mode"`
		SavePath string `yaml:"savePath" env:"SAVE_PATH" env-description:"Path to save urls"`
	} `yaml:"repository"`
	Server struct {
		Address     string `yaml:"address" env:"ADDRESS" env-description:"Address to host"`
		BaseAddress string `yaml:"baseAddress" env:"BASE_ADDRESS" env-description:"Base address for shortlink"`
	} `yaml:"server"`
	Database struct {
		Host     string `yaml:"host" env:"DB_HOS" env-description:"Database host-address"`
		Port     string `yaml:"port" env:"DB_PORT" env-description:"Database port"`
		Dbname   string `yaml:"dbname" env:"DB_NAME" env-description:"Database name"`
		User     string `yaml:"user" env:"DB_USER" env-description:"Database user"`
		Password string `yaml:"password" env:"DB_PASSWORD" env-description:"Database password"`
	} `yaml:"database"`
	Auth struct {
		TokenExp  int    `yaml:"tokenExp" env:"TOKEN_EXP" env-description:"Expire time for token"`
		SecretKey string `yaml:"secretKey" env:"SECRET_KEY" env-description:"Secret key for token"`
	} `yaml:"auth"`
}

func (c *Config) UseDataBase() bool {
	return !c.Repository.InMemory && (c.Database.Host != "")
}

type argsCommandLine struct {
	InMemory         bool
	SavePath         string
	Address          string
	Port             string
	BaseAddress      string
	ConfigPath       string
	Host             string
	DatabasePort     string
	Dbname           string
	DatabaseUser     string
	DatabasePassword string
	TokenExp         string
	SecretKey        string
}

func processArgs(argsToParse []string) (argsCommandLine, error) {
	var a argsCommandLine

	f := flag.NewFlagSet("shortlink", flag.ContinueOnError)
	f.StringVar(
		&a.ConfigPath, "c",
		"/home/work/go/src/github.com/OrtemRepos/shortlink/configs/config.yml",
		"Path to configuration file",
	)
	f.StringVar(&a.SavePath, "s", "", "Path to save data")
	f.BoolVar(&a.InMemory, "im", false, "In-memory mode")
	f.StringVar(&a.Address, "a", "", "Address to host")
	f.StringVar(&a.BaseAddress, "b", "", "Base address for shortlink")
	f.StringVar(&a.Host, "db-address", "", "Database host-address")
	f.StringVar(&a.DatabasePort, "db-port", "", "Database port")
	f.StringVar(&a.Dbname, "db-name", "", "Database name")
	f.StringVar(&a.DatabaseUser, "db-user", "", "Database user")
	f.StringVar(&a.DatabasePassword, "db-password", "", "Database password")
	f.StringVar(&a.TokenExp, "t", "", "Time to expire for token")
	f.StringVar(&a.SecretKey, "sk", "", "Secret key for token")

	f.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		f.PrintDefaults()
	}

	if err := f.Parse(argsToParse); err != nil {
		return a, err
	}
	return a, nil
}

func GetConfig(argsToParse []string) (*Config, error) {
	var cfg Config

	args, err := processArgs(argsToParse)
	if err != nil {
		return nil, err
	}

	log.Printf("Config path: %s", args.ConfigPath)

	if err := cleanenv.ReadConfig(args.ConfigPath, &cfg); err != nil {
		return nil, err
	}

	overrideConfig(&cfg, &args)

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, err
	}

	log.Printf("Save path: %s", cfg.Repository.SavePath)
	log.Printf("Server address: %s", cfg.Server.Address)
	log.Printf("Base address: %s", cfg.Server.BaseAddress)
	log.Printf("Database host-address: %s", cfg.Database.Host)
	log.Printf("Database port: %s", cfg.Database.Port)
	log.Printf("Database name: %s", cfg.Database.Dbname)
	log.Printf("Database user: %s", cfg.Database.User)
	log.Printf("Database password: %s", cfg.Database.Password)

	return &cfg, nil
}

func overrideConfig(cfg *Config, a *argsCommandLine) {
	argsFields := map[string]string{
		"Repository.SavePath": a.SavePath,
		"Server.Address":      a.Address,
		"Server.BaseAddress":  a.BaseAddress,
		"Database.Host":       a.Host,
		"Database.Port":       a.DatabasePort,
		"Database.Dbname":     a.Dbname,
		"Database.User":       a.DatabaseUser,
		"Database.Password":   a.DatabasePassword,
		"Auth.SecretKey":      a.SecretKey,
	}

	if a.InMemory {
		cfg.Repository.InMemory = a.InMemory
	}

	cfgVal := reflect.ValueOf(cfg).Elem()
	for fieldName, val := range argsFields {
		if val != "" {
			setFieldValue(cfgVal, fieldName, val)
		}
	}

	timeExp, err := strconv.Atoi(a.TokenExp)
	if err == nil {
		cfg.Auth.TokenExp = timeExp
	}
	cfg.Auth.TokenExp *= int(time.Second)
}

func setFieldValue(v reflect.Value, fieldName, val string) {
	fields := strings.Split(fieldName, ".")
	for _, f := range fields[:len(fields)-1] {
		fv := v.FieldByName(f)
		if fv.Kind() == reflect.Ptr {
			fv = fv.Elem()
		}
		if fv.Kind() != reflect.Struct {
			panic(fmt.Sprintf("cannot set nested field %s in %s", fieldName, v.Type()))
		}
		v = fv
	}
	lastField := fields[len(fields)-1]
	field := v.FieldByName(lastField)
	if field.Kind() != reflect.String {
		panic(fmt.Sprintf("field %s is not a string", fieldName))
	}
	field.SetString(val)
}

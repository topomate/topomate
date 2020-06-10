package project

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"

	"github.com/docker/distribution/uuid"
	"github.com/mitchellh/go-homedir"
	"github.com/rahveiz/topomate/config"
	"github.com/spf13/viper"
)

var lock sync.Mutex

func Test() {
	u := uuid.Generate()
	fmt.Println(u.String())
	fmt.Println(viper.AllKeys())
}

func getProjectDirectory() string {
	// Check if a directory is configured
	if viper.IsSet("projects_directory") {
		configDir := viper.GetString("projects_directory")

		stat, err := os.Stat(configDir)
		if err == nil {
			if !stat.IsDir() {
				log.Fatalln("projects_directory path is not a directory")
			}
			return configDir
		}

		if os.IsNotExist(err) { // create directory if it is not present yet
			if e := os.Mkdir(configDir, os.ModeDir); e != nil {
				log.Fatalln("error creating projects directory")
			}
			return configDir
		}
		log.Fatalf("configured projects directory error: %v\n", err)
	}

	defaultDir, err := homedir.Expand("~/.topomate")
	if err != nil {
		log.Fatalln(err)
	}

	if _, err := os.Stat(defaultDir); os.IsNotExist(err) {
		if e := os.Mkdir(defaultDir, os.ModeDir|os.ModePerm); e != nil {
			log.Fatalln("error creating projects directory")
		}
	} else if err != nil {
		log.Fatalf("configured projects directory error: %v\n", err)
	}
	return defaultDir
}

func saveToDisk(v interface{}) error {
	lock.Lock()
	defer lock.Unlock()
	projectPath := fmt.Sprintf(
		"%s/%s.json",
		getProjectDirectory(),
		uuid.Generate().String(),
	)

	f, err := os.Create(projectPath)
	if err != nil {
		return err
	}
	defer f.Close()

	b, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return err
	}

	if _, err := f.Write(b); err != nil {
		return err
	}

	return nil
}

func Save(v interface{}) {
	if err := saveToDisk(v); err != nil {
		log.Fatalln(err)
	}
}

func List() {
	d := getProjectDirectory()
	files, err := ioutil.ReadDir(d)
	if err != nil {
		log.Fatalln(err)
	}
	for i, f := range files {
		c := config.BaseConfig{}
		b, err := ioutil.ReadFile(d + "/" + f.Name())
		if err != nil {
			log.Fatalln(err)
		}
		if err := json.Unmarshal(b, &c); err != nil {
			log.Fatalln(err)
		}
		fmt.Printf("(%d)\t%s - %d AS\n", i, c.Name, len(c.As))
	}
}

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
	"github.com/rahveiz/topomate/utils"
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

func getProjectFile(name string) string {
	d := utils.GetDirectoryFromKey("ProjectDir", "")
	filename := fmt.Sprintf("%s/%s.json", d, name)
	return filename
}

func saveToDisk(name string, v interface{}) error {
	filename := getProjectFile(name)
	lock.Lock()
	defer lock.Unlock()

	f, err := os.Create(filename)
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

func Save(name string, v interface{}) {
	if err := saveToDisk(name, v); err != nil {
		log.Fatalln(err)
	}
}

func List() {
	d := utils.GetDirectoryFromKey("ProjectDir", "")
	files, err := ioutil.ReadDir(d)
	if err != nil {
		log.Fatalln(err)
	}
	for _, f := range files {
		c := Project{}
		filename := f.Name()
		b, err := ioutil.ReadFile(d + "/" + filename)
		if err != nil {
			log.Fatalln(err)
		}
		if err := json.Unmarshal(b, &c); err != nil {
			log.Fatalln(err)
		}
		fmt.Printf("(%s)\t%s - %d AS\n",
			filename[:len(filename)-5], c.Name, len(c.AS))
	}
}

func Get(name string) *Project {
	c := &Project{}
	b, err := ioutil.ReadFile(getProjectFile(name))
	if err != nil {
		log.Fatalln(err)
	}

	if err := json.Unmarshal(b, c); err != nil {
		log.Fatalln(err)
	}

	return c
}

func Delete(name string) {
	if err := os.Remove(getProjectFile(name)); err != nil {
		log.Fatalln(err)
	}
}

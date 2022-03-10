package metro

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"gopkg.in/yaml.v2"
)

type Config struct {
	folder string
	lock   sync.RWMutex
	env    map[string]string
	data   map[string]interface{}
	raw    map[string][]byte
}

func New(folder string, env map[string]string) (*Config, error) {
	config := &Config{
		folder: folder,
		lock:   sync.RWMutex{},
		env:    env,
		data:   map[string]interface{}{},
		raw:    map[string][]byte{},
	}
	if _, err := os.Stat(folder); os.IsNotExist(err) {
		return config, nil
	}
	files, err := ioutil.ReadDir(folder)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	for _, file := range files {
		name := file.Name()
		err := config.parse(name)
		if err != nil {
			continue
		}
	}
	watch, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	err = watch.Add(folder)
	if err != nil {
		return nil, err
	}
	go func() {
		defer func() {
			if err := recover(); err != nil {
				fmt.Println(err)
			}
		}()
		for {
			select {
			case ev := <-watch.Events:
				{
					//判断事件发生的类型
					// Create 创建
					// Write 写入
					// Remove 删除
					path, _ := filepath.Abs(ev.Name)
					index := strings.LastIndex(path, string(os.PathSeparator))
					fileName := path[index+1:]
					if ev.Op&fsnotify.Create == fsnotify.Create {
						log.Println("创建文件 : ", ev.Name)
						config.parse(fileName)
					}
					if ev.Op&fsnotify.Write == fsnotify.Write {
						log.Println("写入文件 : ", ev.Name)
						config.parse(fileName)
					}
					if ev.Op&fsnotify.Remove == fsnotify.Remove {
						log.Println("删除文件 : ", ev.Name)
						config.remove(fileName)
					}
				}
			case err := <-watch.Errors:
				{
					log.Println("error : ", err)
					return
				}
			}
		}
	}()
	return config, nil
}

func search(data map[string]interface{}, key []string) interface{} {
	if len(key) == 0 {
		return data
	}
	next, ok := data[key[0]]
	if ok {
		if len(key) == 1 {
			return next
		}
		switch next.(type) {
		case map[interface{}]interface{}:
			return search(cast.ToStringMap(next), key[1:])
		case map[string]interface{}:
			return search(next.(map[string]interface{}), key[1:])
		default:
			return nil
		}
	}
	return nil
}

func replace(content []byte, data map[string]string) []byte {
	if data == nil {
		return content
	}
	for key, val := range data {
		reKey := "env(" + key + ")"
		content = bytes.ReplaceAll(content, []byte(reKey), []byte(val))
	}
	return content
}

func (c *Config) parse(file string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	slice := strings.Split(file, ".")
	if len(slice) == 2 && (slice[1] == "yaml" || slice[1] == "yml") {
		buffer, err := ioutil.ReadFile(filepath.Join(c.folder, file))
		if err != nil {
			return err
		}
		buffer = replace(buffer, c.env)
		data := map[string]interface{}{}
		if err := yaml.Unmarshal(buffer, &data); err != nil {
			return err
		}
		fileName := slice[0]
		c.data[fileName] = data
		c.raw[fileName] = buffer
	}
	return nil
}

func (c *Config) remove(file string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	slice := strings.Split(file, ".")
	if len(slice) == 2 && (slice[1] == "yaml" || slice[1] == "yml") {
		fileName := slice[0]
		delete(c.raw, fileName)
		delete(c.data, fileName)
	}
	return nil
}

func (c *Config) Load(key string, v interface{}) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: "yaml",
		Result:  v,
	})
	if err != nil {
		return err
	}
	return decoder.Decode(c.find(key))
}

func (c *Config) find(key string) interface{} {
	c.lock.Lock()
	defer c.lock.Unlock()
	return search(c.data, strings.Split(key, "."))
}

func (c *Config) IsExist(key string) bool {
	return c.find(key) != nil
}

func (c *Config) Get(key string) interface{} {
	return c.find(key)
}

func (c *Config) GetBool(key string) bool {
	return cast.ToBool(c.find(key))
}

func (c *Config) GetInt(key string) int {
	return cast.ToInt(c.find(key))
}

func (c *Config) GetString(key string) string {
	return cast.ToString(c.find(key))
}

func (c *Config) GetFloat64(key string) float64 {
	return cast.ToFloat64(c.find(key))
}

func (c *Config) GetIntSlice(key string) []int {
	return cast.ToIntSlice(c.find(key))
}

func (c *Config) GetStringSlice(key string) []string {
	return cast.ToStringSlice(c.find(key))
}

func (c *Config) GetStringMap(key string) map[string]interface{} {
	return cast.ToStringMap(c.find(key))
}

func (c *Config) GetStringMapString(key string) map[string]string {
	return cast.ToStringMapString(c.find(key))
}

func (c *Config) GetStringMapStringSlice(key string) map[string][]string {
	return cast.ToStringMapStringSlice(c.find(key))
}

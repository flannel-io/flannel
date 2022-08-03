package common

import (
	"fmt"
    "io/ioutil"
	"strings"

	tcerr "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
)

var (
	globalSectionName = "____GLOBAL____"
	iniErr            = "ClientError.INIError"
)

func openFile(path string) (data []byte, err error) {
	data, err = ioutil.ReadFile(path)
	if err != nil {
		err = tcerr.NewTencentCloudSDKError(iniErr, err.Error(), "")
	}
	return
}

func parse(path string) (*sections, error) {
	result := &sections{map[string]*section{}}
	buf, err := openFile(path)
	if err != nil {
		return &sections{}, err
	}
	content := string(buf)

	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		msg := fmt.Sprintf("the result of reading the %s is empty", path)
		return &sections{}, tcerr.NewTencentCloudSDKError(iniErr, msg, "")
	}
	currentSectionName := globalSectionName
	currentSection := &section{make(map[string]*value)}
	for i, line := range lines {
		line = strings.Replace(line, "\r", "", -1)
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		// comments
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		// section name
		if strings.HasPrefix(line, "[") {
			if strings.HasSuffix(line, "]") {
				tempSection := line[1 : len(line)-1]
				if len(tempSection) == 0 {
					msg := fmt.Sprintf("INI file %s lien %d is not valid: wrong section", path, i)
					return result, tcerr.NewTencentCloudSDKError(iniErr, msg, "")
				}
				// Save the previous section
				result.contains[currentSectionName] = currentSection
				// new section
				currentSectionName = tempSection
				currentSection = &section{make(map[string]*value, 0)}
				continue
			} else {
				msg := fmt.Sprintf("INI file %s lien %d is not valid: wrong section", path, i)
				return result, tcerr.NewTencentCloudSDKError(iniErr, msg, "")
			}
		}

		pos := strings.Index(line, "=")
		if pos > 0 && pos < len(line)-1 {
			key := line[:pos]
			val := line[pos+1:]

			key = strings.TrimSpace(key)
			val = strings.TrimSpace(val)

			v := &value{raw: val}
			currentSection.content[key] = v
		}
	}

	result.contains[currentSectionName] = currentSection
	return result, nil
}

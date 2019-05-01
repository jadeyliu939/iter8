/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package checkandincrement

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

func Invoke(endpoint string, payload Request) (*Response, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	raw, err := http.Post("http://"+endpoint+"/api/v1/analytics/canary/check_and_increment", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	defer raw.Body.Close()
	body, err := ioutil.ReadAll(raw.Body)

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

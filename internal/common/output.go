package common

import (
	"encoding/json"
	"fmt"
)

// OutputJSON marshals the given data to JSON and outputs it to stdout
func OutputJSON(data interface{}) error {
	jsonData, err := ToJSON(data)
	if err != nil {
		return err
	}

	fmt.Println(string(jsonData))
	return nil
}

// ToJSON marshals the given data to JSON
func ToJSON(data interface{}) ([]byte, error) {
	return json.MarshalIndent(data, "", "  ")
}
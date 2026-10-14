package cibudgetdomain

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"syscall"
)

const journalLimit = 4 * 1024 * 1024

func ReadObservations(name string) ([]Observation, error) {
	file, err := os.Open(name)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() > journalLimit {
		return nil, fmt.Errorf("timing journal exceeds 4 MiB")
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 65536)
	result := []Observation{}
	ids := map[string]bool{}
	for scanner.Scan() {
		var observation Observation
		if err = json.Unmarshal(scanner.Bytes(), &observation); err != nil {
			return nil, err
		}
		if observation.ID == "" || ids[observation.ID] || len(result) >= 4096 {
			return nil, fmt.Errorf("timing journal identity or population differs")
		}
		ids[observation.ID] = true
		result = append(result, observation)
	}
	return result, scanner.Err()
}
func Estimates(observations []Observation) (map[string]Estimate, error) {
	groups := map[string][]Observation{}
	for _, observation := range observations {
		groups[observation.CostKey] = append(groups[observation.CostKey], observation)
	}
	result := map[string]Estimate{}
	for key, group := range groups {
		estimate, err := Calibrate(key, group)
		if err != nil {
			return nil, err
		}
		result[key] = estimate
	}
	return result, nil
}

// Append preserves timing origins; a full journal is reported, never truncated.
func AppendObservations(name string, observations []Observation) error {
	file, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	old, err := ReadObservations(name)
	if err != nil {
		return err
	}
	combined := append(old, observations...)
	ids := map[string]bool{}
	for _, observation := range combined {
		if ids[observation.ID] {
			return fmt.Errorf("timing origin already exists")
		}
		ids[observation.ID] = true
	}
	if len(combined) > 4096 {
		return fmt.Errorf("timing journal population exceeds 4096")
	}
	if _, err = Estimates(combined); err != nil {
		return err
	}
	encoded := []byte{}
	for _, observation := range observations {
		data, marshalErr := json.Marshal(observation)
		if marshalErr != nil {
			return marshalErr
		}
		encoded = append(encoded, data...)
		encoded = append(encoded, '\n')
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size()+int64(len(encoded)) > journalLimit {
		return fmt.Errorf("timing journal exceeds 4 MiB")
	}
	if _, err = file.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	if _, err = file.Write(encoded); err != nil {
		return err
	}
	return file.Sync()
}

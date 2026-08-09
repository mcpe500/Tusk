package engine

import (
	"errors"
	"fmt"
	"time"

	"github.com/mcpe500/Tusk/internal/state"
)

const AcceptanceSchemaVersion = 1

type AcceptanceRecord struct {
	ID         string    `json:"id"`
	Passed     bool      `json:"passed"`
	Device     string    `json:"device"`
	Evidence   string    `json:"evidence"`
	RecordedAt time.Time `json:"recordedAt"`
}

type AcceptanceMatrix struct {
	SchemaVersion int                `json:"schemaVersion"`
	Records       []AcceptanceRecord `json:"records"`
}

func NewAcceptanceMatrix() AcceptanceMatrix {
	return AcceptanceMatrix{SchemaVersion: AcceptanceSchemaVersion}
}

func (matrix AcceptanceMatrix) Record(record AcceptanceRecord) (AcceptanceMatrix, error) {
	if matrix.SchemaVersion == 0 {
		matrix.SchemaVersion = AcceptanceSchemaVersion
	}
	if record.ID == "" || record.Device == "" || record.Evidence == "" {
		return AcceptanceMatrix{}, errors.New("acceptance record requires ID, device, and evidence")
	}
	record.RecordedAt = time.Now().UTC()
	for index, existing := range matrix.Records {
		if existing.ID == record.ID && existing.Device == record.Device {
			matrix.Records[index] = record
			return matrix, nil
		}
	}
	matrix.Records = append(matrix.Records, record)
	return matrix, nil
}

func (matrix AcceptanceMatrix) ValidateMVP() error {
	if matrix.SchemaVersion != AcceptanceSchemaVersion {
		return errors.New("unsupported acceptance matrix schema")
	}
	seen := make(map[string]bool)
	for _, record := range matrix.Records {
		if record.Passed {
			if record.Device == "" || record.Evidence == "" {
				return errors.New("passed acceptance evidence requires device and evidence provenance")
			}
			seen[record.ID] = true
		}
	}
	for number := 1; number <= 36; number++ {
		id := fmt.Sprintf("AT-%03d", number)
		if !seen[id] {
			return fmt.Errorf("MVP acceptance evidence missing or failed for %s", id)
		}
	}
	return nil
}

func SaveAcceptanceMatrix(path string, matrix AcceptanceMatrix) error {
	if matrix.SchemaVersion != AcceptanceSchemaVersion {
		return errors.New("unsupported acceptance matrix schema")
	}
	return state.WriteJSON(path, matrix, 0o600)
}

func LoadAcceptanceMatrix(path string) (AcceptanceMatrix, error) {
	var matrix AcceptanceMatrix
	if err := state.ReadJSON(path, &matrix); err != nil {
		return AcceptanceMatrix{}, err
	}
	if matrix.SchemaVersion != AcceptanceSchemaVersion {
		return AcceptanceMatrix{}, errors.New("unsupported acceptance matrix schema")
	}
	return matrix, nil
}

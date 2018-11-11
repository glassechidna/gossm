package gossm

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/aws/aws-sdk-go/service/ssm"
	_ "github.com/mattn/go-sqlite3"
	"os/user"
	"path/filepath"
)

type History struct {
	db *sql.DB
}

func NewHistory(path string) (*History, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`create table if not exists commands (commandId text primary key, commandJson text, invocations text, complete bool);`)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`create table if not exists invocations (commandId text, instanceId text, stdout text, stderr text, primary key (commandId, instanceId));`)
	if err != nil {
		return nil, err
	}

	return &History{
		db: db,
	}, nil
}

func (h *History) Close() error {
	return h.db.Close()
}

func (h *History) PutCommand(status Status) error {
	commandId := *status.Command.CommandId

	bytes, err := json.Marshal(status.Command)
	if err != nil {
		return err
	}
	_, err = h.db.Exec(`insert into commands (commandId, commandJson) values(?, ?) on conflict(commandId) do update set commandJson = excluded.commandJson`, commandId, bytes)
	if err != nil {
		return err
	}

	bytes, _ = json.Marshal(status.Invocations)
	_, err = h.db.Exec(`insert into commands (commandId, invocations) values(?, ?) on conflict(commandId) do update set invocations = excluded.Invocations`, commandId, bytes)
	if err != nil {
		return err
	}

	return nil
}

func (h *History) AppendPayload(msg SsmMessage) error {
	_, err := h.db.Exec(`
		insert into Invocations (commandId, instanceId, stdout, stderr) 
		values (?, ?, ?, ?) 
		on conflict (commandId, instanceId) do update set stdout = stdout || excluded.stdout, stderr = stderr || excluded.stderr
	`, msg.CommandId, msg.Payload.InstanceId, msg.Payload.StdoutChunk, msg.Payload.StderrChunk)
	return err
}

type HistoricalStatus struct {
	*Status
}

func rowToStatus(rows *sql.Rows) (*HistoricalStatus, error) {
	var commandJson, invocationsJson []byte
	err := rows.Scan(&commandJson, &invocationsJson)
	if err != nil {
		return nil, err
	}

	command := ssm.Command{}
	err = json.Unmarshal(commandJson, &command)
	if err != nil {
		return nil, err
	}

	invocations := Invocations{}
	err = json.Unmarshal(invocationsJson, &invocations)
	if err != nil {
		return nil, err
	}

	return &HistoricalStatus{&Status{Command: &command, Invocations: invocations}}, nil
}

func (h *History) Command(commandId string) (*HistoricalStatus, error) {
	rows, err := h.db.Query(`select commandJson, invocations from commands where commandId = ?`, commandId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()


	for rows.Next() {
		return rowToStatus(rows)
	}

	return nil, errors.New("no command by id " + commandId)
}

func (h *History) Commands() ([]HistoricalStatus, error) {
	rows, err := h.db.Query(`select commandJson, invocations from commands`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var commands []HistoricalStatus

	for rows.Next() {
		status, err := rowToStatus(rows)
		if err != nil {
			return nil, err
		}
		commands = append(commands, *status)
	}

	return commands, nil
}

type HistoricalOutput struct {
	CommandId  string
	InstanceId string
	Stdout     string
	Stderr     string
}

func (h *History) CommandOutputs(commandId string) ([]HistoricalOutput, error) {
	rows, err := h.db.Query(`select instanceId, stdout, stderr from invocations where commandId = ?`, commandId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var outputs []HistoricalOutput

	for rows.Next() {
		output := HistoricalOutput{}
		err = rows.Scan(&output.InstanceId, &output.Stdout, &output.Stderr)
		if err != nil {
			return nil, err
		}

		outputs = append(outputs, output)
	}

	return outputs, nil
}

func (c *HistoricalStatus) Stream(outputs []HistoricalOutput, ch chan SsmMessage) {
	defer close(ch)

	for _, o := range outputs {
		ch <- SsmMessage{
			CommandId: o.CommandId,
			Payload: &SsmPayloadMessage{
				InstanceId:  o.InstanceId,
				StdoutChunk: o.Stdout,
				StderrChunk: o.Stderr,
			},
		}
	}

	ch <- SsmMessage{
		CommandId: *c.Command.CommandId,
		Control: &SsmControlMessage{
			Status: c.Status,
		},
	}
}

var DefaultHistory *History

func init() {
	u, _ := user.Current()
	path := filepath.Join(u.HomeDir, ".gossm.history.db")
	DefaultHistory, _ = NewHistory(path)
}

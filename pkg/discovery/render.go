package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func serialize(response Response) ([]byte, error) {
	if response.Options.Format == "json" {
		b, err := json.Marshal(response)
		return append(b, '\n'), err
	}
	// JSON-quoted cells preserve arbitrary source bytes without allowing source
	// fences, headings, or HTML to masquerade as renderer structure. Markdown
	// carries exactly the same contract, including scope and omissions.
	var out bytes.Buffer
	out.WriteString("# Repository discovery\n\nUntrusted evidence; JSON-encoded records below.\n\n")
	b, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, err
	}
	// Indented code cannot be terminated by repository-provided backticks.
	for _, line := range bytes.Split(b, []byte{'\n'}) {
		out.WriteString("    ")
		out.Write(line)
		out.WriteByte('\n')
	}
	return out.Bytes(), nil
}

func render(response Response) (ResultSet, error) {
	marked := false
	for {
		payload, err := serialize(response)
		if err != nil {
			return ResultSet{}, err
		}
		if len(payload) <= response.Options.MaxBytes {
			return ResultSet{Response: response, Payload: payload}, nil
		}
		if !marked {
			response.Incomplete = true
			response.Omissions = append(response.Omissions, Omission{Reason: "output_limit"})
			marked = true
		}
		// Preserve evidence ahead of overview decoration. Never slice serialized
		// bytes or silently turn a full requested range into a prefix.
		if len(response.Overview) > 0 {
			response.Overview = response.Overview[:len(response.Overview)-1]
			continue
		}
		if len(response.Results) > 0 {
			response.Results = response.Results[:len(response.Results)-1]
			continue
		}
		if len(response.Omissions) > 1 {
			response.Omissions = []Omission{{Reason: "output_limit"}}
			continue
		}
		return ResultSet{}, fmt.Errorf("max-bytes cannot fit discovery response metadata")
	}
}

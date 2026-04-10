package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"
)

// ListProjects lists all deployed projects
func ListProjects(ctx context.Context, serverURL string) error {
	fmt.Printf("Fetching projects from %s ...\n", serverURL)
	req, err := http.NewRequestWithContext(ctx, "GET", serverURL+"/projects", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var result struct {
		Projects []struct {
			Name       string `json:"name"`
			URL        string `json:"url"`
			DeployedAt string `json:"deployed_at"`
		} `json:"projects"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprintln(w, "NAME\tURL\tDEPLOYED_AT")
	for _, p := range result.Projects {
		fmt.Fprintf(w, "%s\t%s\t%s\n", p.Name, p.URL, p.DeployedAt)
	}
	w.Flush()
	return nil
}

// ViewProject shows details for a specific project
func ViewProject(ctx context.Context, serverURL, name string) error {
	fmt.Printf("Fetching project '%s' from %s ...\n", name, serverURL)
	req, err := http.NewRequestWithContext(ctx, "GET", serverURL+"/projects/"+name, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("project '%s' not found", name)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var result map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &result)

	fmt.Printf("Name: %s\nURL: %s\nFiles: %v\nDeployed: %v\n",
		result["name"], result["url"], result["files"], result["deployed_at"])
	return nil
}

// DeleteProject deletes a project
func DeleteProject(ctx context.Context, serverURL, name string) error {
	fmt.Printf("Deleting project '%s' from %s ...\n", name, serverURL)
	req, err := http.NewRequestWithContext(ctx, "DELETE", serverURL+"/projects/"+name, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}
	fmt.Printf("Deleted: %s\n", name)
	return nil
}

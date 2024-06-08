package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cheggaaa/pb/v3"
	"github.com/cli/go-gh/v2/pkg/api"
)

var sizes = []string{"B", "kB", "MB", "GB", "TB", "PB", "EB"}

var repoName, tagName string

type Release struct {
	URL             string    `json:"url,omitempty"`
	AssetsURL       string    `json:"assets_url,omitempty"`
	UploadURL       string    `json:"upload_url,omitempty"`
	HTMLURL         string    `json:"html_url,omitempty"`
	ID              int       `json:"id,omitempty"`
	NodeID          string    `json:"node_id,omitempty"`
	TagName         string    `json:"tag_name,omitempty"`
	TargetCommitish string    `json:"target_commitish,omitempty"`
	Name            string    `json:"name,omitempty"`
	Draft           bool      `json:"draft,omitempty"`
	Prerelease      bool      `json:"prerelease,omitempty"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
	PublishedAt     time.Time `json:"published_at,omitempty"`
	// Assets          []Assets  `json:"assets,omitempty"`
	Assets []struct {
		URL                string    `json:"url,omitempty"`
		ID                 int       `json:"id,omitempty"`
		NodeID             string    `json:"node_id,omitempty"`
		Name               string    `json:"name,omitempty"`
		Label              string    `json:"label,omitempty"`
		ContentType        string    `json:"content_type,omitempty"`
		State              string    `json:"state,omitempty"`
		Size               int       `json:"size,omitempty"`
		DownloadCount      int       `json:"download_count,omitempty"`
		CreatedAt          time.Time `json:"created_at,omitempty"`
		UpdatedAt          time.Time `json:"updated_at,omitempty"`
		BrowserDownloadURL string    `json:"browser_download_url,omitempty"`
	}
	TarballURL string `json:"tarball_url,omitempty"`
	ZipballURL string `json:"zipball_url,omitempty"`
}

func getReleaseInfo(repoName, tagName string) Release {
	client, err := api.DefaultRESTClient()
	if err != nil {
		fmt.Println(err)
	}

	response := Release{}
	var url string
	if tagName == "latest" {
		url = fmt.Sprintf("repos/%s/releases/latest", repoName)
	} else {
		url = fmt.Sprintf("repos/%s/releases/tags/%s", repoName, tagName)
	}

	err = client.Get(url, &response)
	if err != nil {
		fmt.Println(err)
	}
	return response
}

func renderTable() table.Model {
	release := getReleaseInfo(repoName, tagName)

	columns := []table.Column{
		{Title: "Asset Name", Width: 50},
		{Title: "Size", Width: 20},
	}

	var rows []table.Row

	for _, assetname := range release.Assets {
		rows = append(rows, table.Row{assetname.Name, humanReadableSize(float64(assetname.Size), 1024.0)})
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
	)

	t.SetStyles(createTableStyles())

	return t
}

func humanReadableSize(s float64, base float64) string {
	unitsLimit := len(sizes)
	i := 0
	for s >= base && i < unitsLimit {
		s = s / base
		i++
	}

	f := "%.0f %s"
	if i > 1 {
		f = "%.2f %s"
	}

	return fmt.Sprintf(f, s, sizes[i])
}

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type model struct {
	table table.Model
	rlz   Release
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.table.Focused() {
				m.table.Blur()
			} else {
				m.table.Focus()
			}
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			return m, tea.Batch(
				tea.Printf("Download URL: curl -LO %s", m.rlz.Assets[m.table.Cursor()].BrowserDownloadURL),
			)
		case "d":
			selectedAsset := m.rlz.Assets[m.table.Cursor()]
			go func() {
				err := downloadFile(selectedAsset.BrowserDownloadURL)
				if err != nil {
					log.Fatalf("error downloading file: %v\n", err)
				}
			}()
		}
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return baseStyle.Render(m.table.View()) + "\n"
}

func createTableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	return s
}

func main() {
	flag.StringVar(&repoName, "repo", "kubernetes-sigs/cluster-api", "GitHub repository name")
	flag.StringVar(&repoName, "R", "kubernetes-sigs/cluster-api", "GitHub repository name")
	flag.StringVar(&tagName, "tag", "latest", "GitHub release tag")
	flag.StringVar(&tagName, "t", "latest", "GitHub release tag")
	flag.Parse()

	m := model{renderTable(), getReleaseInfo(repoName, tagName)}
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Error running program:", err)
	}
}

func downloadFile(url string) error {
	out, err := os.Create(url[strings.LastIndex(url, "/")+1:])
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create progress bar
	tmp := `{{string . "prefix"}}{{counters . }} {{bar . "[" "=" ">" "-" "]"}} {{percent . }} {{etime . }}{{string . "suffix"}}`
	bar := pb.Full.Start64(resp.ContentLength)
	bar.Set(pb.Bytes, true)
	bar.SetTemplate(pb.ProgressBarTemplate(tmp))

	reader := bar.NewProxyReader(resp.Body)

	_, err = io.Copy(out, reader)
	if err != nil {
		return err
	}

	bar.Finish()

	return nil
}

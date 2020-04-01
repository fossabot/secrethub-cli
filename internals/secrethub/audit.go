package secrethub

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/secrethub/secrethub-go/pkg/secrethub/iterator"

	"github.com/secrethub/secrethub-cli/internals/cli/ui"
	"github.com/secrethub/secrethub-cli/internals/secrethub/command"
	"github.com/secrethub/secrethub-go/pkg/secrethub"

	"github.com/secrethub/secrethub-go/internals/api"
)

var (
	errPagerNotFound = errors.New("no terminal pager available. Please configure a terminal pager by setting the $PAGER environment variable or install \"less\" or \"more\"")
)

const (
	pagerEnvvar            = "$PAGER"
	defaultTerminalWidth   = 80
	fallbackPagerLineCount = 100
)

// AuditCommand is a command to audit a repo or a secret.
type AuditCommand struct {
	io                 ui.IO
	newPaginatedWriter func(io.Writer) (pager, error)
	path               api.Path
	useTimestamps      bool
	timeFormatter      TimeFormatter
	newClient          newClientFunc
	terminalWidth      func(int) (int, error)
	perPage            int
	json               bool
}

// NewAuditCommand creates a new audit command.
func NewAuditCommand(io ui.IO, newClient newClientFunc) *AuditCommand {
	return &AuditCommand{
		io:                 io,
		newPaginatedWriter: newPaginatedWriter,
		newClient:          newClient,
		terminalWidth: func(fd int) (int, error) {
			w, _, err := terminal.GetSize(fd)
			return w, err
		},
	}
}

// Register registers the command, arguments and flags on the provided Registerer.
func (cmd *AuditCommand) Register(r command.Registerer) {
	clause := r.Command("audit", "Show the audit log.\n\nIf the output of the command is parsed by a script an alternative of the default table format must be used.")
	clause.Arg("repo-path or secret-path", "Path to the repository or the secret to audit "+repoPathPlaceHolder+" or "+secretPathPlaceHolder).SetValue(&cmd.path)
	clause.Flag("per-page", "number of audit events shown per page").Default("20").Hidden().IntVar(&cmd.perPage)
	clause.Flag("json", "output the audit log in json format").BoolVar(&cmd.json)
	registerTimestampFlag(clause).BoolVar(&cmd.useTimestamps)

	command.BindAction(clause, cmd.Run)
}

// Run prints all audit events for the given repository or secret.
func (cmd *AuditCommand) Run() error {
	cmd.beforeRun()
	return cmd.run()
}

// beforeRun configures the command using the flag values.
func (cmd *AuditCommand) beforeRun() {
	cmd.timeFormatter = NewTimeFormatter(cmd.useTimestamps)
}

// Run prints all audit events for the given repository or secret.
func (cmd *AuditCommand) run() error {
	if cmd.perPage < 1 {
		return fmt.Errorf("per-page should be positive, got %d", cmd.perPage)
	}

	iter, auditTable, err := cmd.iterAndAuditTable()
	if err != nil {
		return err
	}

	var formatter tableFormatter
	if cmd.json {
		formatter = newJSONFormatter(auditTable.header())
	} else {
		terminalWidth, err := cmd.terminalWidth(int(os.Stdout.Fd()))
		if err != nil {
			terminalWidth = defaultTerminalWidth
		}
		formatter = newColumnFormatter(terminalWidth, auditTable.columns())
	}

	paginatedWriter, err := cmd.newPaginatedWriter(os.Stdout)
	if err == errPagerNotFound {
		paginatedWriter = newFallbackPager(os.Stdout)
	} else if err != nil {
		return err
	}
	defer paginatedWriter.Close()

	if formatter.printHeader() {
		header, err := formatter.formatRow(auditTable.header())
		if err != nil {
			return err
		}
		fmt.Fprintln(paginatedWriter, header)
	}

	for {
		event, err := iter.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return err
		}

		row, err := auditTable.row(event)
		if err != nil {
			return err
		}

		formattedRow, err := formatter.formatRow(row)
		if err != nil {
			return err
		}

		if paginatedWriter.IsClosed() {
			break
		}
		_, err = fmt.Fprintln(paginatedWriter, formattedRow)
		if err != nil {
			return err
		}
	}
	return nil
}

type tableFormatter interface {
	printHeader() bool
	formatRow(row []string) (string, error)
}

// newJSONFormatter returns a table formatter that formats the given table rows as json.
func newJSONFormatter(fieldNames []string) *jsonFormatter {
	return &jsonFormatter{fields: fieldNames}
}

type jsonFormatter struct {
	fields []string
}

func (f *jsonFormatter) printHeader() bool {
	return false
}

// formatRow returns the json representation of the given row
// with the configured field names as keys and the provided values
func (f *jsonFormatter) formatRow(row []string) (string, error) {
	if len(f.fields) != len(row) {
		return "", fmt.Errorf("unexpected number of json fields")
	}

	jsonMap := make(map[string]string)
	for i, element := range row {
		jsonMap[f.fields[i]] = element
	}

	jsonData, err := json.Marshal(jsonMap)
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}

// newColumnFormatter returns a table formatter that aligns the columns of the table.
func newColumnFormatter(tableWidth int, columns []auditTableColumn) *columnFormatter {
	return &columnFormatter{tableWidth: tableWidth, columns: columns}
}

type columnFormatter struct {
	tableWidth           int
	computedColumnWidths []int
	columns              []auditTableColumn
}

func (f *columnFormatter) printHeader() bool {
	return true
}

// formatRow formats the given table row to fit the configured width by
// giving each cell an equal width and wrapping the text in cells that exceed it.
func (f *columnFormatter) formatRow(row []string) (string, error) {
	columnWidths := f.columnWidths()

	// calculate the maximum number of lines a cell value will be broken into
	maxLinesPerCell := 1
	for i, cell := range row {
		lines := len(cell) / columnWidths[i]
		if len(cell)%columnWidths[i] != 0 {
			lines++
		}
		if lines > maxLinesPerCell {
			maxLinesPerCell = lines
		}
	}

	// split the cell values into a grid according to how they will be printed
	splitCells := make([][]string, maxLinesPerCell)
	for i := 0; i < maxLinesPerCell; i++ {
		splitCells[i] = make([]string, len(row))
	}

	for i, cell := range row {
		columnWidth := columnWidths[i]
		lineCount := len(cell) / columnWidth
		for j := 0; j < lineCount; j++ {
			begin := j * columnWidth
			end := (j + 1) * columnWidth
			splitCells[j][i] = cell[begin:end]
		}

		charactersLeft := len(cell) % columnWidth
		if charactersLeft != 0 {
			splitCells[lineCount][i] = cell[len(cell)-charactersLeft:] + strings.Repeat(" ", columnWidth-charactersLeft)
		} else if lineCount < maxLinesPerCell {
			splitCells[lineCount][i] = strings.Repeat(" ", columnWidth)
		}

		for j := lineCount + 1; j < maxLinesPerCell; j++ {
			splitCells[j][i] = strings.Repeat(" ", columnWidth)
		}
	}

	// convert the grid to a string
	strRes := strings.Builder{}
	for j := 0; j < maxLinesPerCell; j++ {
		strRes.WriteString(strings.Join(splitCells[j], "  ") + "\n")
	}
	return strings.TrimSuffix(strRes.String(), "\n"), nil
}

// columnWidths returns the width of each column based on their maximum widths
// and the table width.
func (f *columnFormatter) columnWidths() []int {
	if f.computedColumnWidths != nil {
		return f.computedColumnWidths
	}
	res := make([]int, len(f.columns))

	// Distribute the maximum width equally between all columns and repeatedly
	// check if any of them have a smaller maximum width and can be shrunk.
	// Stop when no columns can be further adjusted.
	adjusted := true
	columnsLeft := len(f.columns)
	widthLeft := f.tableWidth - 2*(len(f.columns)-1)
	widthPerColumn := widthLeft / columnsLeft
	for adjusted {
		adjusted = false
		for i, col := range f.columns {
			if res[i] == 0 && col.maxWidth != 0 && col.maxWidth < widthPerColumn {
				res[i] = col.maxWidth
				widthLeft -= col.maxWidth
				columnsLeft--
				adjusted = true
			}
		}
		if columnsLeft == 0 {
			for i := range res {
				res[i] += widthLeft / len(res)
			}
			break
		}
		widthPerColumn = widthLeft / columnsLeft
	}

	// distribute the remaining width equally between columns with no maximum width.
	for i := range res {
		if res[i] == 0 {
			res[i] = widthPerColumn
		}
	}
	f.computedColumnWidths = res
	return res
}

func (cmd *AuditCommand) iterAndAuditTable() (secrethub.AuditEventIterator, auditTable, error) {
	repoPath, err := cmd.path.ToRepoPath()
	if err == nil {
		client, err := cmd.newClient()
		if err != nil {
			return nil, nil, err
		}
		tree, err := client.Dirs().GetTree(repoPath.GetDirPath().Value(), -1, false)
		if err != nil {
			return nil, nil, err
		}

		iter := client.Repos().EventIterator(repoPath.Value(), &secrethub.AuditEventIteratorParams{})
		auditTable := newRepoAuditTable(tree, cmd.timeFormatter)
		return iter, auditTable, nil

	}

	secretPath, err := cmd.path.ToSecretPath()
	if err == nil {
		if cmd.path.HasVersion() {
			return nil, nil, ErrCannotAuditSecretVersion
		}

		client, err := cmd.newClient()
		if err != nil {
			return nil, nil, err
		}

		isDir, err := client.Dirs().Exists(secretPath.Value())
		if err == nil && isDir {
			return nil, nil, ErrCannotAuditDir
		}

		iter := client.Secrets().EventIterator(secretPath.Value(), &secrethub.AuditEventIteratorParams{})
		auditTable := newSecretAuditTable(cmd.timeFormatter)
		return iter, auditTable, nil
	}

	return nil, nil, ErrNoValidRepoOrSecretPath
}

type pager interface {
	io.WriteCloser
	IsClosed() bool
}

// newPaginatedWriter runs the terminal pager configured in the OS environment
// and returns a writer that is piped to the standard input of the pager command.
func newPaginatedWriter(outputWriter io.Writer) (pager, error) {
	pager, err := pagerCommand()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(pager)

	writer, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	cmd.Stdout = outputWriter
	cmd.Stderr = os.Stderr

	err = cmd.Start()
	if err != nil {
		return nil, err
	}
	done := make(chan struct{}, 1)
	go func() {
		_ = cmd.Wait()
		done <- struct{}{}
	}()
	return &paginatedWriter{writer: writer, cmd: cmd, done: done}, nil
}

// paginatedWriter is a writer that is piped to a terminal pager command.
type paginatedWriter struct {
	writer io.WriteCloser
	cmd    *exec.Cmd
	done   <-chan struct{}
	closed bool
}

func (p *paginatedWriter) Write(data []byte) (n int, err error) {
	return p.writer.Write(data)
}

// Close closes the writer to the terminal pager and waits for the terminal pager to close.
func (p *paginatedWriter) Close() error {
	err := p.writer.Close()
	if err != nil {
		return err
	}
	if !p.closed {
		<-p.done
	}
	return nil
}

// IsClosed checks if the terminal pager process has been stopped.
func (p *paginatedWriter) IsClosed() bool {
	if p.closed {
		return true
	}
	select {
	case <-p.done:
		p.closed = true
		return true
	default:
		return false
	}
}

// pagerCommand returns the name of the terminal pager configured in the OS environment ($PAGER).
// If no pager is configured it falls back to "less" than "more", returning an error if neither are available.
func pagerCommand() (string, error) {
	var pager string
	var err error

	pager, err = exec.LookPath(os.ExpandEnv(pagerEnvvar))
	if err == nil {
		return pager, nil
	}

	pager, err = exec.LookPath("less")
	if err == nil {
		return pager, nil
	}

	pager, err = exec.LookPath("more")
	if err == nil {
		return pager, nil
	}

	return "", errPagerNotFound
}

// newFallbackPager returns a pager that outputs a fixed number of lines without pagination
// and returns errPagerNotFound on the last (or any subsequent) write.
func newFallbackPager(w io.WriteCloser) pager {
	return &fallbackPager{
		linesLeft: fallbackPagerLineCount,
		writer:    w,
	}
}

type fallbackPager struct {
	writer    io.WriteCloser
	linesLeft int
}

func (p *fallbackPager) Write(data []byte) (int, error) {
	if p.linesLeft == 0 {
		return 0, errPagerNotFound
	}

	lines := bytes.Count(data, []byte{'\n'})
	if lines > p.linesLeft {
		data = bytes.Join(bytes.Split(data, []byte{'\n'})[:p.linesLeft], []byte{'\n'})
		data = append(data, '\n')
	}
	p.linesLeft -= bytes.Count(data, []byte{'\n'})
	n, err := p.writer.Write(data)
	if p.linesLeft == 0 {
		err = errPagerNotFound
	}
	return n, err
}

func (p *fallbackPager) Close() error {
	return p.writer.Close()
}

func (p *fallbackPager) IsClosed() bool {
	return p.linesLeft == 0
}

type auditTableColumn struct {
	name     string
	maxWidth int
}

type auditTable interface {
	header() []string
	row(event api.Audit) ([]string, error)
	columns() []auditTableColumn
}

func newBaseAuditTable(timeFormatter TimeFormatter, midColumns ...auditTableColumn) baseAuditTable {
	columns := append([]auditTableColumn{
		{name: "AUTHOR", maxWidth: 32},
		{name: "EVENT", maxWidth: 22},
	}, midColumns...)
	columns = append(columns, []auditTableColumn{
		{name: "IP ADDRESS", maxWidth: 45},
		{name: "DATE", maxWidth: 22},
	}...)

	return baseAuditTable{
		tableColumns:  columns,
		timeFormatter: timeFormatter,
	}
}

type baseAuditTable struct {
	tableColumns  []auditTableColumn
	timeFormatter TimeFormatter
}

func (table baseAuditTable) header() []string {
	res := make([]string, len(table.tableColumns))
	for i, col := range table.tableColumns {
		res[i] = col.name
	}
	return res
}

func (table baseAuditTable) row(event api.Audit, content ...string) ([]string, error) {
	actor, err := getAuditActor(event)
	if err != nil {
		return nil, err
	}

	res := append([]string{actor, getEventAction(event)}, content...)
	return append(res, event.IPAddress, table.timeFormatter.Format(event.LoggedAt)), nil
}

func (table baseAuditTable) columns() []auditTableColumn {
	return table.tableColumns
}

func newSecretAuditTable(timeFormatter TimeFormatter) secretAuditTable {
	return secretAuditTable{
		baseAuditTable: newBaseAuditTable(timeFormatter),
	}
}

type secretAuditTable struct {
	baseAuditTable
}

func (table secretAuditTable) header() []string {
	return table.baseAuditTable.header()
}

func (table secretAuditTable) row(event api.Audit) ([]string, error) {
	return table.baseAuditTable.row(event)
}

func newRepoAuditTable(tree *api.Tree, timeFormatter TimeFormatter) repoAuditTable {
	return repoAuditTable{
		baseAuditTable: newBaseAuditTable(timeFormatter, auditTableColumn{name: "EVENT SUBJECT"}),
		tree:           tree,
	}
}

type repoAuditTable struct {
	baseAuditTable
	tree *api.Tree
}

func (table repoAuditTable) row(event api.Audit) ([]string, error) {
	subject, err := getAuditSubject(event, table.tree)
	if err != nil {
		return nil, err
	}

	return table.baseAuditTable.row(event, subject)
}

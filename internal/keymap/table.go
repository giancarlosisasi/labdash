package keymap

import "github.com/giancarlosisasi/labdash/internal/action"

// Every action labdash will ever have, as an identifier. They are declared as
// constants so that a surface names one and a typo is a compile error.
const (
	// Movement and the frame.
	Down        Action = "down"
	Up          Action = "up"
	PrevSection Action = "prev-section"
	NextSection Action = "next-section"
	FirstRow    Action = "first-row"
	LastRow     Action = "last-row"
	PageDown    Action = "page-down"
	PageUp      Action = "page-up"
	NextView    Action = "next-view"
	PrevView    Action = "prev-view"
	FocusPane   Action = "focus-preview"
	Back        Action = "back"
	Help        Action = "help"
	Quit        Action = "quit"
	LoadMore    Action = "load-more"

	// The three that replaced the config file.
	Browse Action = "browse"
	Filter Action = "filter"
	Pin    Action = "pin"

	// Universal actions.
	Refresh        Action = "refresh"
	RefreshAll     Action = "refresh-all"
	TogglePreview  Action = "toggle-preview"
	OpenInBrowser  Action = "open-in-browser"
	OpenInEditor   Action = "open-in-editor"
	CopyRef        Action = "copy-ref"
	CopyURL        Action = "copy-url"
	Search         Action = "search"
	SearchNext     Action = "search-next"
	SearchPrev     Action = "search-prev"
	Collapse       Action = "collapse"
	CollapseAll    Action = "collapse-all"
	MultiSelect    Action = "multi-select"
	Logs           Action = "logs"
	Undo           Action = "undo"
	PrevPreviewTab Action = "prev-preview-tab"
	NextPreviewTab Action = "next-preview-tab"

	// Chords.
	CommandPalette Action = "command-palette"
	JumpToProject  Action = "jump-to-project"
	ScopeToRepo    Action = "scope-to-repo"
	SwitchInstance Action = "switch-instance"

	// Quick-filter chips. Digits are reserved for these and are never view or
	// tab selectors.
	QuickFilter1 Action = "quick-filter-1"
	QuickFilter2 Action = "quick-filter-2"
	QuickFilter3 Action = "quick-filter-3"
	QuickFilter4 Action = "quick-filter-4"
	QuickFilter5 Action = "quick-filter-5"

	// Browse.
	BrowseExpand   Action = "browse-expand"
	BrowseCollapse Action = "browse-collapse"
	BrowseScope    Action = "browse-scope"

	// Merge requests.
	Approve           Action = "approve"
	ApproveSelected   Action = "approve-selected"
	Merge             Action = "merge"
	Comment           Action = "comment"
	Checkout          Action = "checkout"
	CheckoutWorktree  Action = "checkout-worktree"
	ToggleDraft       Action = "toggle-draft"
	RetryHeadPipeline Action = "retry-head-pipeline"
	Labels            Action = "labels"
	Assign            Action = "assign"
	Unassign          Action = "unassign"
	RequestReview     Action = "request-review"
	CloseMR           Action = "close-mr"
	ReopenMR          Action = "reopen-mr"
	UpdateBranch      Action = "update-branch"
	NewMR             Action = "new-merge-request"

	// Pipelines and jobs.
	RetryPipeline      Action = "retry-pipeline"
	CancelPipeline     Action = "cancel-pipeline"
	PlayManualJob      Action = "play-manual-job"
	WatchPipeline      Action = "watch-pipeline"
	StageGraph         Action = "stage-graph"
	PipelineVariables  Action = "pipeline-variables"
	RunPipeline        Action = "run-pipeline"
	ScheduledPipelines Action = "scheduled-pipelines"

	// Job trace viewer.
	TraceFollow Action = "trace-follow"
	TraceCopy   Action = "trace-copy"
	TraceSave   Action = "trace-save"
	RetryJob    Action = "retry-job"

	// To-Dos.
	ToDoDone    Action = "todo-done"
	ToDoDoneAll Action = "todo-done-all"
	ToDoSnooze  Action = "todo-snooze"
	ToDoGoTo    Action = "todo-go-to-target"
)

// The verbs. One concept, one key — a verb bound to two different keys fails
// the coherence check, which is the checkable form of "a key means the same
// verb in every view".
const (
	verbMoveDown       Verb = "move down"
	verbMoveUp         Verb = "move up"
	verbPrevSection    Verb = "show the previous section"
	verbNextSection    Verb = "show the next section"
	verbFirstRow       Verb = "jump to the first row"
	verbLastRow        Verb = "jump to the last row"
	verbPageDown       Verb = "page down"
	verbPageUp         Verb = "page up"
	verbNextView       Verb = "switch to the next view"
	verbPrevView       Verb = "switch to the previous view"
	verbFocusPane      Verb = "focus the preview"
	verbBack           Verb = "go back"
	verbHelp           Verb = "show the help"
	verbQuit           Verb = "quit"
	verbLoadMore       Verb = "load more"
	verbBrowse         Verb = "browse"
	verbFilter         Verb = "filter"
	verbPin            Verb = "pin"
	verbRefresh        Verb = "refresh"
	verbRefreshAll     Verb = "refresh everything"
	verbTogglePreview  Verb = "toggle the preview"
	verbOpen           Verb = "open it in a browser"
	verbOpenInEditor   Verb = "open it in an editor"
	verbCopyRef        Verb = "copy the ref"
	verbCopyURL        Verb = "copy the URL"
	verbSearch         Verb = "search"
	verbSearchNext     Verb = "go to the next match"
	verbSearchPrev     Verb = "go to the previous match"
	verbCollapse       Verb = "collapse"
	verbCollapseAll    Verb = "collapse everything"
	verbMultiSelect    Verb = "select"
	verbLogs           Verb = "show the logs"
	verbUndo           Verb = "undo"
	verbPrevTab        Verb = "show the previous preview tab"
	verbNextTab        Verb = "show the next preview tab"
	verbPalette        Verb = "open the palette"
	verbJumpToProject  Verb = "jump to a project"
	verbScopeToRepo    Verb = "scope to this repository"
	verbSwitchInstance Verb = "switch instance"
	verbQuickFilter    Verb = "apply quick filter "
	verbExpandNode     Verb = "expand this node"
	verbCollapseNode   Verb = "collapse this node"
	verbScopeHere      Verb = "scope the dashboard here"
	verbApprove        Verb = "approve"
	verbApproveAll     Verb = "approve the selection"
	verbMerge          Verb = "merge"
	verbComment        Verb = "comment"
	verbCheckout       Verb = "check out the branch"
	verbWorktree       Verb = "check it out as a worktree"
	verbToggleDraft    Verb = "change the draft state"
	verbRetry          Verb = "retry"
	verbLabels         Verb = "edit the labels"
	verbAssign         Verb = "assign"
	verbUnassign       Verb = "unassign"
	verbRequestReview  Verb = "send it for review"
	verbStop           Verb = "stop it"
	verbReopen         Verb = "reopen"
	verbUpdateBranch   Verb = "update the branch"
	verbCreate         Verb = "create"
	verbPlay           Verb = "play it"
	verbWatch          Verb = "watch"
	verbStageGraph     Verb = "show the stage graph"
	verbVariables      Verb = "show the variables"
	verbSchedules      Verb = "show the schedules"
	verbCopyLog        Verb = "copy the log"
	verbSave           Verb = "save it"
	verbDone           Verb = "mark it done"
	verbDoneAll        Verb = "mark them all done"
	verbSnooze         Verb = "snooze it"
	verbGoToTarget     Verb = "go to the target"
)

// quickFilter is five near-identical rows, written once. Each chip is its own
// verb: they are five different filters, not one verb behind five keys.
func quickFilter(n int, id Action) Entry {
	digit := string(rune('0' + n))
	return Entry{
		ID: id, Key: digit, Tier: L2, Verb: verbQuickFilter + Verb(digit),
		Help:   "apply quick filter " + digit,
		Screen: action.Everywhere,
	}
}

// table is the whole keymap.
//
// The order here is the order the published cheatsheet prints. Implemented
// marks what this build does; everything else is bound, published, and answers
// when pressed.
var table = []Entry{
	// ---------------------------------------------------------------------
	// Universal — movement. Every vim key carries its non-vim alternate: nobody
	// is locked out for not knowing vim, and nobody who knows it has to reach
	// for an arrow key.
	// ---------------------------------------------------------------------
	{ID: Down, Key: "j", Alt: []string{"down"}, Tier: L1, Verb: verbMoveDown,
		Help: "move down a row", Screen: action.Everywhere, Implemented: true},
	{ID: Up, Key: "k", Alt: []string{"up"}, Tier: L1, Verb: verbMoveUp,
		Help: "move up a row", Screen: action.Everywhere, Implemented: true},
	{ID: PrevSection, Key: "h", Alt: []string{"left"}, Tier: L1, Verb: verbPrevSection,
		Help: "previous section", Screen: action.Everywhere, Implemented: true},
	{ID: NextSection, Key: "l", Alt: []string{"right"}, Tier: L1, Verb: verbNextSection,
		Help: "next section", Screen: action.Everywhere, Implemented: true},
	{ID: FirstRow, Key: "g", Alt: []string{"home"}, Tier: L1, Verb: verbFirstRow,
		Help: "jump to the first row", Short: "first", Screen: action.Everywhere, Implemented: true},
	{ID: LastRow, Key: "G", Alt: []string{"end"}, Tier: L1, Verb: verbLastRow,
		Help: "jump to the last row", Short: "last", Screen: action.Everywhere, Implemented: true,
		Escalates: FirstRow},
	{ID: PageDown, Key: "ctrl+d", Alt: []string{"pgdown"}, Tier: L1, Verb: verbPageDown,
		Help: "page down", Screen: action.Everywhere, Implemented: true},
	{ID: PageUp, Key: "ctrl+u", Alt: []string{"pgup"}, Tier: L1, Verb: verbPageUp,
		Help: "page up", Screen: action.Everywhere, Implemented: true},
	{ID: NextView, Key: "tab", Tier: L0, Verb: verbNextView,
		Help: "next view", Screen: action.Everywhere, Implemented: true},
	{ID: PrevView, Key: "shift+tab", Tier: L0, Verb: verbPrevView,
		Help: "previous view", Screen: action.Everywhere, Implemented: true},
	{ID: FocusPane, Key: "enter", Tier: L0, Verb: verbFocusPane,
		Help: "focus the preview pane", Short: "focus", Screen: action.Everywhere},
	{ID: Back, Key: "esc", Tier: L0, Verb: verbBack,
		Help: "go back one level", Screen: action.Everywhere, Implemented: true},
	{ID: Help, Key: "?", Tier: L0, Verb: verbHelp,
		Help: "show the keys for this view", Screen: action.Everywhere, Implemented: true},
	{ID: Quit, Key: "q", Tier: L0, Verb: verbQuit,
		Help: "quit labdash", Screen: action.Everywhere, Implemented: true},
	{ID: LoadMore, Key: "J", Tier: L2, Verb: verbLoadMore,
		Help: "load the next page of rows", Short: "more", Screen: action.Everywhere, Escalates: Down},

	// ---------------------------------------------------------------------
	// The three keys that are the product. They lead the overlay and the page
	// because together they replace the config file.
	// ---------------------------------------------------------------------
	{ID: Browse, Key: "b", Tier: L0, Verb: verbBrowse,
		Help: "browse your whole GitLab", Screen: action.Everywhere,
		Requires: action.Requirement{Network: true}},
	{ID: Filter, Key: "f", Tier: L0, Verb: verbFilter,
		Help: "filter what you are looking at", Screen: action.Everywhere},
	{ID: Pin, Key: "P", Tier: L0, Verb: verbPin,
		Help: "pin this view as a permanent tab", Screen: action.Everywhere},

	// ---------------------------------------------------------------------
	// Universal — actions.
	// ---------------------------------------------------------------------
	{ID: Refresh, Key: "r", Tier: L2, Verb: verbRefresh,
		Help: "refresh this section", Screen: action.Everywhere,
		Requires: action.Requirement{Network: true}},
	{ID: RefreshAll, Key: "ctrl+r", Tier: L3, Verb: verbRefreshAll,
		Help: "refresh every section", Screen: action.Everywhere, Escalates: Refresh,
		Requires: action.Requirement{Network: true}},
	{ID: TogglePreview, Key: "p", Tier: L2, Verb: verbTogglePreview,
		Help: "show or hide the preview", Short: "preview", Screen: action.Everywhere},
	{ID: OpenInBrowser, Key: "o", Tier: L2, Verb: verbOpen,
		Help: "open it in your browser", Short: "open", Screen: action.Everywhere},
	{ID: OpenInEditor, Key: "O", Tier: L2, Verb: verbOpenInEditor,
		Help: "open it in your editor", Short: "editor", Screen: action.Everywhere, Escalates: OpenInBrowser},
	{ID: CopyRef, Key: "y", Tier: L2, Verb: verbCopyRef,
		Help: "copy the branch or ref", Short: "copy ref", Screen: action.Everywhere},
	{ID: CopyURL, Key: "Y", Tier: L2, Verb: verbCopyURL,
		Help: "copy the URL", Short: "copy URL", Screen: action.Everywhere, Escalates: CopyRef},
	{ID: Search, Key: "/", Tier: L1, Verb: verbSearch,
		Help: "search what is on screen", Screen: action.Everywhere},
	{ID: SearchNext, Key: "n", Tier: L1, Verb: verbSearchNext,
		Help: "go to the next match", Short: "next match", Screen: action.Everywhere,
		Requires: action.Requirement{SearchLive: true}},
	{ID: SearchPrev, Key: "N", Tier: L1, Verb: verbSearchPrev,
		Help: "go to the previous match", Short: "prev match", Screen: action.Everywhere, Escalates: SearchNext,
		Requires: action.Requirement{SearchLive: true}},
	{ID: Collapse, Key: "z", Tier: L2, Verb: verbCollapse,
		Help: "collapse this group", Screen: action.Everywhere},
	{ID: CollapseAll, Key: "Z", Tier: L2, Verb: verbCollapseAll,
		Help: "collapse every group", Screen: action.Everywhere, Escalates: Collapse},
	{ID: MultiSelect, Key: "space", Tier: L2, Verb: verbMultiSelect,
		Help: "add this row to the selection", Short: "select", Screen: action.Everywhere},
	{ID: Logs, Key: "L", Tier: L2, Verb: verbLogs,
		Help: "show the logs", Screen: action.Everywhere},
	{ID: Undo, Key: "u", Tier: L2, Verb: verbUndo,
		Help: "undo the last reversible action", Screen: action.Everywhere,
		Requires: action.Requirement{Write: true, Network: true}},
	{ID: PrevPreviewTab, Key: "[", Tier: L2, Verb: verbPrevTab,
		Help: "previous preview tab", Short: "prev tab", Screen: action.Everywhere},
	{ID: NextPreviewTab, Key: "]", Tier: L2, Verb: verbNextTab,
		Help: "next preview tab", Short: "next tab", Screen: action.Everywhere},

	// ---------------------------------------------------------------------
	// Universal — chords. Ctrl-prefixed and instantaneous. Ctrl+S, Ctrl+I,
	// Ctrl+M and Ctrl+C are never bound: XOFF freezes the display, and the
	// other three arrive as Tab, Enter and interrupt.
	// ---------------------------------------------------------------------
	{ID: CommandPalette, Key: "ctrl+k", Tier: L3, Verb: verbPalette,
		Help: "find any action by name", Short: "palette", Screen: action.Everywhere},
	{ID: JumpToProject, Key: "ctrl+p", Tier: L3, Verb: verbJumpToProject,
		Help: "jump to a project", Short: "jump", Screen: action.Everywhere,
		Requires: action.Requirement{Network: true}},
	{ID: ScopeToRepo, Key: "ctrl+g", Tier: L3, Verb: verbScopeToRepo,
		Help: "scope to the repository you are in", Short: "this repo", Screen: action.Everywhere},
	{ID: SwitchInstance, Key: "ctrl+t", Tier: L3, Verb: verbSwitchInstance,
		Help: "switch GitLab instance", Short: "instance", Screen: action.Everywhere},

	quickFilter(1, QuickFilter1),
	quickFilter(2, QuickFilter2),
	quickFilter(3, QuickFilter3),
	quickFilter(4, QuickFilter4),
	quickFilter(5, QuickFilter5),

	// ---------------------------------------------------------------------
	// Browse. h and l walk the tree here rather than switching section tabs,
	// because Browse has no section tabs. It is the one context-dependent
	// meaning in the whole keymap, and it is the universal tree convention.
	// ---------------------------------------------------------------------
	{ID: BrowseExpand, Key: "l", Alt: []string{"right"}, Tier: L1, Verb: verbExpandNode,
		Help: "expand this node", Short: "expand", Screen: action.Browse, Overrides: NextSection,
		Requires: action.Requirement{Row: action.TreeNodeRow}},
	{ID: BrowseCollapse, Key: "h", Alt: []string{"left"}, Tier: L1, Verb: verbCollapseNode,
		Help: "collapse this node", Short: "collapse", Screen: action.Browse, Overrides: PrevSection,
		Requires: action.Requirement{Row: action.TreeNodeRow}},
	{ID: BrowseScope, Key: "enter", Tier: L0, Verb: verbScopeHere,
		Help: "scope the dashboard to this node", Short: "scope here", Screen: action.Browse, Overrides: FocusPane,
		Requires: action.Requirement{Row: action.TreeNodeRow}},

	// ---------------------------------------------------------------------
	// Merge requests.
	// ---------------------------------------------------------------------
	{ID: Approve, Key: "v", Tier: L2, Verb: verbApprove,
		Help: "approve it, or take your approval back", Screen: action.MergeRequests,
		Requires: action.Requirement{Row: action.MergeRequestRow, Write: true, Network: true}},
	{ID: ApproveSelected, Key: "V", Tier: L2, Verb: verbApproveAll,
		Help: "approve every selected row", Short: "approve all", Screen: action.MergeRequests, Escalates: Approve,
		Requires: action.Requirement{Row: action.MergeRequestRow, Write: true, Network: true}},
	{ID: Merge, Key: "m", Tier: L2, Verb: verbMerge,
		Help: "merge it", Screen: action.MergeRequests,
		Requires: action.Requirement{Row: action.MergeRequestRow, Write: true, Network: true}},
	{ID: Comment, Key: "c", Tier: L2, Verb: verbComment,
		Help: "write a comment", Screen: action.MergeRequests,
		Requires: action.Requirement{Row: action.MergeRequestRow, Write: true, Network: true}},
	{ID: Checkout, Key: "C", Tier: L2, Verb: verbCheckout,
		Help: "check the branch out locally", Short: "checkout", Screen: action.MergeRequests,
		Requires: action.Requirement{Row: action.MergeRequestRow}},
	{ID: CheckoutWorktree, Key: "W", Tier: L2, Verb: verbWorktree,
		Help: "check it out as a worktree", Short: "worktree", Screen: action.MergeRequests,
		Requires: action.Requirement{Row: action.MergeRequestRow}},
	{ID: ToggleDraft, Key: "d", Tier: L2, Verb: verbToggleDraft,
		Help: "mark it draft, or mark it ready", Short: "draft", Screen: action.MergeRequests,
		Requires: action.Requirement{Row: action.MergeRequestRow, Write: true, Network: true}},
	{ID: RetryHeadPipeline, Key: "R", Tier: L2, Verb: verbRetry,
		Help: "retry the head pipeline", Short: "retry", Screen: action.MergeRequests,
		Requires: action.Requirement{Row: action.MergeRequestRow, Write: true, Network: true}},
	{ID: Labels, Key: "t", Tier: L2, Verb: verbLabels,
		Help: "edit the labels", Screen: action.MergeRequests,
		Requires: action.Requirement{Row: action.MergeRequestRow, Write: true, Network: true}},
	{ID: Assign, Key: "a", Tier: L2, Verb: verbAssign,
		Help: "assign someone", Screen: action.MergeRequests,
		Requires: action.Requirement{Row: action.MergeRequestRow, Write: true, Network: true}},
	{ID: Unassign, Key: "A", Tier: L2, Verb: verbUnassign,
		Help: "unassign someone", Screen: action.MergeRequests, Escalates: Assign,
		Requires: action.Requirement{Row: action.MergeRequestRow, Write: true, Network: true}},
	{ID: RequestReview, Key: "s", Tier: L2, Verb: verbRequestReview,
		Help: "send it for review", Short: "review", Screen: action.MergeRequests,
		Requires: action.Requirement{Row: action.MergeRequestRow, Write: true, Network: true}},
	{ID: CloseMR, Key: "x", Tier: L2, Verb: verbStop,
		Help: "close it", Short: "close", Screen: action.MergeRequests,
		Requires: action.Requirement{Row: action.MergeRequestRow, Write: true, Network: true}},
	{ID: ReopenMR, Key: "X", Tier: L2, Verb: verbReopen,
		Help: "reopen it", Short: "reopen", Screen: action.MergeRequests, Escalates: CloseMR,
		Requires: action.Requirement{Row: action.MergeRequestRow, Write: true, Network: true}},
	{ID: UpdateBranch, Key: "U", Tier: L2, Verb: verbUpdateBranch,
		Help: "update the branch from its target", Short: "update", Screen: action.MergeRequests,
		Requires: action.Requirement{Row: action.MergeRequestRow, Write: true, Network: true}},
	{ID: NewMR, Key: "ctrl+n", Tier: L3, Verb: verbCreate,
		Help: "start a new merge request", Short: "new", Screen: action.MergeRequests,
		Requires: action.Requirement{Write: true, Network: true}},

	// ---------------------------------------------------------------------
	// Pipelines and jobs.
	// ---------------------------------------------------------------------
	{ID: RetryPipeline, Key: "R", Tier: L2, Verb: verbRetry,
		Help: "retry it", Short: "retry", Screen: action.Pipelines,
		Requires: action.Requirement{Row: action.PipelineRow, Write: true, Network: true}},
	{ID: CancelPipeline, Key: "x", Tier: L2, Verb: verbStop,
		Help: "cancel it", Short: "cancel", Screen: action.Pipelines,
		Requires: action.Requirement{Row: action.PipelineRow, Write: true, Network: true}},
	{ID: PlayManualJob, Key: "m", Tier: L2, Verb: verbPlay,
		Help: "play a manual job", Short: "play", Screen: action.Pipelines,
		Requires: action.Requirement{Row: action.PipelineRow, Write: true, Network: true}},
	{ID: WatchPipeline, Key: "w", Tier: L2, Verb: verbWatch,
		Help: "keep watching it as it runs", Short: "watch", Screen: action.Pipelines,
		Requires: action.Requirement{Row: action.PipelineRow, Network: true}},
	{ID: StageGraph, Key: "D", Tier: L2, Verb: verbStageGraph,
		Help: "show the stage graph", Short: "graph", Screen: action.Pipelines,
		Requires: action.Requirement{Row: action.PipelineRow}},
	{ID: PipelineVariables, Key: "V", Tier: L2, Verb: verbVariables,
		Help: "show the variables it ran with", Short: "variables", Screen: action.Pipelines,
		Requires: action.Requirement{Row: action.PipelineRow}},
	{ID: ScheduledPipelines, Key: "S", Tier: L2, Verb: verbSchedules,
		Help: "show the scheduled pipelines", Short: "schedules", Screen: action.Pipelines,
		Requires: action.Requirement{Network: true}},
	{ID: RunPipeline, Key: "ctrl+n", Tier: L3, Verb: verbCreate,
		Help: "run a pipeline", Short: "run", Screen: action.Pipelines,
		Requires: action.Requirement{Write: true, Network: true}},

	// ---------------------------------------------------------------------
	// Job trace viewer. w follows a growing log, which is the same verb as
	// watching a running pipeline — one word for "keep showing me this as it
	// changes", in both places.
	// ---------------------------------------------------------------------
	{ID: TraceFollow, Key: "w", Tier: L2, Verb: verbWatch,
		Help: "follow the log as it grows", Short: "follow", Screen: action.Trace,
		Requires: action.Requirement{Network: true}},
	{ID: TraceCopy, Key: "y", Tier: L2, Verb: verbCopyLog,
		Help: "copy the log", Short: "copy", Screen: action.Trace, Overrides: CopyRef},
	{ID: TraceSave, Key: "S", Tier: L2, Verb: verbSave,
		Help: "save it to a file", Short: "save", Screen: action.Trace},
	{ID: RetryJob, Key: "R", Tier: L2, Verb: verbRetry,
		Help: "retry this job", Short: "retry", Screen: action.Trace,
		Requires: action.Requirement{Row: action.JobRow, Write: true, Network: true}},

	// ---------------------------------------------------------------------
	// To-Dos. u restores a To-Do, which is undo doing its job rather than a
	// second binding for the same idea, so it is declared once, universally.
	// ---------------------------------------------------------------------
	{ID: ToDoDone, Key: "d", Tier: L2, Verb: verbDone,
		Help: "mark it done", Short: "done", Screen: action.ToDos,
		Requires: action.Requirement{Row: action.ToDoRow, Write: true, Network: true}},
	{ID: ToDoDoneAll, Key: "D", Tier: L2, Verb: verbDoneAll,
		Help: "mark every To-Do done", Short: "done all", Screen: action.ToDos, Escalates: ToDoDone,
		Requires: action.Requirement{Write: true, Network: true}},
	{ID: ToDoSnooze, Key: "s", Tier: L2, Verb: verbSnooze,
		Help: "snooze it", Short: "snooze", Screen: action.ToDos,
		Requires: action.Requirement{Row: action.ToDoRow, Write: true, Network: true}},
	{ID: ToDoGoTo, Key: "enter", Tier: L0, Verb: verbGoToTarget,
		Help: "go to what it points at", Short: "go to", Screen: action.ToDos, Overrides: FocusPane,
		Requires: action.Requirement{Row: action.ToDoRow}},
}

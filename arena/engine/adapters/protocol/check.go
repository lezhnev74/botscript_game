package protocol

// ConfigBody is the answer of GET /api/config.
type ConfigBody struct {
	// BaseURL is the public address of the server (env ARENA_BASE_URL), "" when not configured.
	BaseURL string `json:"baseUrl"`
}

// CheckError is one problem of a program sent to /api/check.
type CheckError struct {
	Line    int    `json:"line"`
	Col     int    `json:"col"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

// CheckActions counts the actions the bot chose in the trial, by kind.
type CheckActions struct {
	Scan int `json:"scan"`
	Move int `json:"move"`
	Fire int `json:"fire"`
	Wait int `json:"wait"`
}

// CheckTrial is what a valid program did in the trial match.
type CheckTrial struct {
	Beats     int          `json:"beats"`
	DeadBeats int          `json:"deadBeats"`
	Actions   CheckActions `json:"actions"`
	Idle      int          `json:"idle"`     // beats missed: the code spent its op budget
	NoAction  int          `json:"noAction"` // beats made a wait because loop() returned without an action
	Clicks    int          `json:"clicks"`   // fire with the gun not loaded
	HitsTaken int          `json:"hitsTaken"`
	Hits      int          `json:"hits"` // own bullets that landed
	Kills     int          `json:"kills"`
	Deaths    int          `json:"deaths"`
	Coins     int          `json:"coins"`
	Warnings  []string     `json:"warnings"`
}

// CheckResponse is the answer of /api/check: Errors for a bad program (Ok
// false), Trial for a good one (Ok true).
type CheckResponse struct {
	Ok     bool         `json:"ok"`
	Errors []CheckError `json:"errors,omitempty"`
	Trial  *CheckTrial  `json:"trial,omitempty"`
}

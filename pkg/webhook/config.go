package webhook

// BeforeConfig controls a synchronous callback that runs before a business action.
type BeforeConfig struct {
	Enable         bool `yaml:"enable" json:"enable"`
	Timeout        int  `yaml:"timeout" json:"timeout"`
	FailedContinue bool `yaml:"failedContinue" json:"failedContinue"`
}

// AfterConfig controls an asynchronous callback that runs after a business action.
type AfterConfig struct {
	Enable  bool `yaml:"enable" json:"enable"`
	Timeout int  `yaml:"timeout" json:"timeout"`
}

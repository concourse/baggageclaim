module github.com/concourse/baggageclaim

replace github.com/concourse/flag => /Users/clarafu/workspace/flag

require (
	code.cloudfoundry.org/lager v1.1.0
	github.com/bmizerany/pat v0.0.0-20170815010413-6226ea591a40 // indirect
	github.com/cenkalti/backoff v2.1.1+incompatible
	github.com/clarafu/envstruct v0.0.0-20210217164029-cb472d46e597
	github.com/concourse/flag v0.0.0-20180907155614-cb47f24fff1c
	github.com/concourse/go-archive v0.0.0-20180803203406-784931698f4f
	github.com/concourse/retryhttp v0.0.0-20170802173037-937335fd9545
	github.com/go-playground/locales v0.13.0
	github.com/go-playground/universal-translator v0.17.0
	github.com/go-playground/validator/v10 v10.4.1
	github.com/google/go-cmp v0.4.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1
	github.com/jessevdk/go-flags v1.4.0
	github.com/klauspost/compress v1.9.7
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/spf13/cobra v1.1.3
	github.com/stretchr/testify v1.6.1
	github.com/tedsuo/ifrit v0.0.0-20180802180643-bea94bb476cc
	github.com/tedsuo/rata v1.0.1-0.20170830210128-07d200713958
	golang.org/x/sys v0.0.0-20190624142023-c5567b49c5d0
	gopkg.in/yaml.v2 v2.4.0
)

go 1.13

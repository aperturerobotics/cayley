module github.com/cayleygraph/cayley

go 1.19

replace github.com/cayleygraph/quad => github.com/paralin/cayley-quad v1.2.5-0.20230429052655-3e19050a092d // aperture

// cayley has not been updated to support v0.2.0
require github.com/hidal-go/hidalgo v0.0.0-20190814174001-42e03f3b5eaa

require (
	github.com/badgerodon/peg v0.0.0-20130729175151-9e5f7f4d07ca
	github.com/cayleygraph/quad v1.2.4
	github.com/cznic/mathutil v0.0.0-20181122101859-297441e03548
	github.com/dennwc/graphql v0.4.19-0.20180603144102-12cfed44bc5d
	github.com/dop251/goja v0.0.0-20230427124612-428fc442ff5f
	github.com/fsouza/go-dockerclient v1.9.7
	github.com/go-sql-driver/mysql v1.7.1
	github.com/gobuffalo/packr/v2 v2.8.3
	github.com/golang/glog v1.1.1
	github.com/golang/protobuf v1.5.3
	github.com/jackc/pgx v3.6.2+incompatible
	github.com/julienschmidt/httprouter v1.3.0
	github.com/lib/pq v1.10.9
	github.com/mattn/go-sqlite3 v2.0.3+incompatible
	github.com/mr-tron/base58 v1.2.0
	github.com/peterh/liner v1.2.2
	github.com/phayes/freeport v0.0.0-20220201140144-74d24b5ae9f5
	github.com/piprate/json-gold v0.5.0
	github.com/spf13/cobra v1.7.0
	github.com/spf13/viper v1.15.0
	github.com/stretchr/testify v1.8.2
	github.com/syndtr/goleveldb v1.0.0
	github.com/tylertreat/BoomFilters v0.0.0-20210315201527-1a82519a3e43
	github.com/vmihailenco/msgpack/v5 v5.3.5
	google.golang.org/protobuf v1.30.0
)

require (
	github.com/AndreasBriese/bbloom v0.0.0-20190306092124-e2d15f34fcf9 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Microsoft/go-winio v0.6.0 // indirect
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/boltdb/bolt v1.3.1 // indirect
	github.com/cenkalti/backoff v2.1.1+incompatible // indirect
	github.com/cockroachdb/apd v1.1.0 // indirect
	github.com/containerd/containerd v1.6.18 // indirect
	github.com/containerd/continuity v0.3.0 // indirect
	github.com/coreos/bbolt v1.3.2 // indirect
	github.com/d4l3k/messagediff v1.2.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dennwc/base v1.0.0 // indirect
	github.com/dgraph-io/badger v1.5.4 // indirect
	github.com/dgryski/go-farm v0.0.0-20190416075124-e1214b5e05dc // indirect
	github.com/dlclark/regexp2 v1.7.0 // indirect
	github.com/docker/docker v23.0.2+incompatible // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/flimzy/kivik v1.8.1 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-kivik/couchdb v1.8.1 // indirect
	github.com/go-kivik/kivik v1.8.1 // indirect
	github.com/go-kivik/pouchdb v1.3.5 // indirect
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible // indirect
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/gobuffalo/logger v1.0.6 // indirect
	github.com/gobuffalo/packd v1.0.1 // indirect
	github.com/gofrs/uuid v4.4.0+incompatible // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v0.0.1 // indirect
	github.com/google/pprof v0.0.0-20230207041349-798e818bf904 // indirect
	github.com/google/uuid v1.2.0 // indirect
	github.com/gopherjs/gopherjs v0.0.0-20190411002643-bd77b112433e // indirect
	github.com/gopherjs/jsbuiltin v0.0.0-20180426082241-50091555e127 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/fake v0.0.0-20150926172116-812a484cc733 // indirect
	github.com/karrick/godirwalk v1.16.1 // indirect
	github.com/klauspost/compress v1.11.13 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mailru/easyjson v0.0.0-20190403194419-1ea4449da983 // indirect
	github.com/markbates/errx v1.1.0 // indirect
	github.com/markbates/oncer v1.0.0 // indirect
	github.com/markbates/safe v1.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.3 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/moby/patternmatcher v0.5.0 // indirect
	github.com/moby/sys/sequential v0.5.0 // indirect
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.3-0.20211202183452-c5a74bcca799 // indirect
	github.com/opencontainers/runc v1.1.5 // indirect
	github.com/ory/dockertest v3.3.4+incompatible // indirect
	github.com/pborman/uuid v1.2.0 // indirect
	github.com/pelletier/go-toml/v2 v2.0.6 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/pquerna/cachecontrol v0.0.0-20180517163645-1555304b9b35 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rogpeppe/go-internal v1.8.0 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spf13/afero v1.9.3 // indirect
	github.com/spf13/cast v1.5.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/subosito/gotenv v1.4.2 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/xdg/scram v0.0.0-20180814205039-7eeb5667e42c // indirect
	github.com/xdg/stringprep v1.0.0 // indirect
	go.mongodb.org/mongo-driver v1.0.4 // indirect
	golang.org/x/crypto v0.0.0-20220525230936-793ad666bf5e // indirect
	golang.org/x/mod v0.8.0 // indirect
	golang.org/x/net v0.9.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/sys v0.7.0 // indirect
	golang.org/x/term v0.7.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	golang.org/x/tools v0.6.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/olivere/elastic.v5 v5.0.80 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

module github.com/cayleygraph/cayley

go 1.19

replace github.com/cayleygraph/quad => github.com/paralin/cayley-quad v1.2.5-0.20230429050549-1ed30505b980 // vtprotobuf

require (
	github.com/badgerodon/peg v0.0.0-20130729175151-9e5f7f4d07ca
	github.com/cayleygraph/quad v1.2.4
	github.com/cznic/mathutil v0.0.0-20170313102836-1447ad269d64
	github.com/dennwc/graphql v0.0.0-20180603144102-12cfed44bc5d
	github.com/dop251/goja v0.0.0-20190105122144-6d5bf35058fa
	github.com/fsouza/go-dockerclient v1.2.2
	github.com/go-sql-driver/mysql v1.4.1
	github.com/gobuffalo/packr/v2 v2.7.1
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.5.2
	github.com/hidal-go/hidalgo v0.0.0-20190814174001-42e03f3b5eaa
	github.com/jackc/pgx v3.3.0+incompatible
	github.com/julienschmidt/httprouter v1.2.0
	github.com/lib/pq v1.1.1
	github.com/mattn/go-sqlite3 v1.10.0
	github.com/peterh/liner v0.0.0-20170317030525-88609521dc4b
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/piprate/json-gold v0.5.0
	github.com/spf13/cobra v0.0.5
	github.com/spf13/viper v1.4.0
	github.com/stretchr/testify v1.8.2
	github.com/syndtr/goleveldb v1.0.0
	github.com/tylertreat/BoomFilters v0.0.0-20181028192813-611b3dbe80e8
	golang.org/x/net v0.0.0-20190628185345-da137c7871d7
	google.golang.org/appengine v1.6.1
)

require (
	github.com/AndreasBriese/bbloom v0.0.0-20190306092124-e2d15f34fcf9 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/Microsoft/go-winio v0.4.12 // indirect
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/boltdb/bolt v1.3.1 // indirect
	github.com/cenkalti/backoff v2.1.1+incompatible // indirect
	github.com/cockroachdb/apd v1.1.0 // indirect
	github.com/containerd/continuity v0.0.0-20190426062206-aaeac12a7ffc // indirect
	github.com/coreos/bbolt v1.3.3 // indirect
	github.com/d4l3k/messagediff v1.2.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dennwc/base v1.0.0 // indirect
	github.com/dgraph-io/badger v1.5.5 // indirect
	github.com/dgryski/go-farm v0.0.0-20190423205320-6a90982ecee2 // indirect
	github.com/dlclark/regexp2 v1.1.4 // indirect
	github.com/docker/docker v0.7.3-0.20180412203414-a422774e593b // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/flimzy/diff v0.1.6 // indirect
	github.com/flimzy/kivik v1.8.1 // indirect
	github.com/fsnotify/fsnotify v1.4.7 // indirect
	github.com/go-kivik/couchdb v1.8.1 // indirect
	github.com/go-kivik/kivik v1.8.1 // indirect
	github.com/go-kivik/pouchdb v1.3.5 // indirect
	github.com/go-sourcemap/sourcemap v2.1.2+incompatible // indirect
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/gobuffalo/envy v1.7.1 // indirect
	github.com/gobuffalo/logger v1.0.1 // indirect
	github.com/gobuffalo/packd v0.3.0 // indirect
	github.com/gogo/protobuf v1.2.1 // indirect
	github.com/golang/snappy v0.0.1 // indirect
	github.com/google/uuid v1.1.1 // indirect
	github.com/gopherjs/gopherjs v0.0.0-20190430165422-3e4dfb77656c // indirect
	github.com/gopherjs/jsbuiltin v0.0.0-20180426082241-50091555e127 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jackc/fake v0.0.0-20150926172116-812a484cc733 // indirect
	github.com/joho/godotenv v1.3.0 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/magiconair/properties v1.8.1 // indirect
	github.com/mailru/easyjson v0.0.0-20190626092158-b2ccc519800e // indirect
	github.com/mitchellh/mapstructure v1.1.2 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/opencontainers/runc v0.1.1 // indirect
	github.com/opencontainers/selinux v1.0.0 // indirect
	github.com/ory/dockertest v3.3.4+incompatible // indirect
	github.com/pborman/uuid v1.2.0 // indirect
	github.com/pelletier/go-toml v1.4.0 // indirect
	github.com/pkg/errors v0.8.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/pquerna/cachecontrol v0.0.0-20180517163645-1555304b9b35 // indirect
	github.com/rogpeppe/go-internal v1.5.0 // indirect
	github.com/satori/go.uuid v1.2.0 // indirect
	github.com/shopspring/decimal v0.0.0-20180709203117-cd690d0c9e24 // indirect
	github.com/sirupsen/logrus v1.4.2 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/cast v1.3.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/xdg/scram v0.0.0-20180814205039-7eeb5667e42c // indirect
	github.com/xdg/stringprep v1.0.0 // indirect
	go.etcd.io/bbolt v1.3.3 // indirect
	go.mongodb.org/mongo-driver v1.0.4 // indirect
	golang.org/x/crypto v0.0.0-20191002192127-34f69633bfdc // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e // indirect
	golang.org/x/sys v0.0.0-20191009170203-06d7bd2c5f4f // indirect
	golang.org/x/text v0.3.2 // indirect
	golang.org/x/tools v0.0.0-20191010075000-0337d82405ff // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/olivere/elastic.v5 v5.0.81 // indirect
	gopkg.in/yaml.v2 v2.2.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/Sirupsen/logrus => github.com/Sirupsen/logrus v1.0.1

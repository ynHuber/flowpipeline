module codeberg.org/BelWue/flowpipeline

go 1.24.10

replace codeberg.org/BelWue/flowpipeline => .

require (
	github.com/BelWue/bgp_routeinfo v1.0.0
	github.com/BelWue/flowfilter v1.0.0
	github.com/ClickHouse/clickhouse-go/v2 v2.40.3
	github.com/IBM/sarama v1.46.3
	github.com/Yawning/cryptopan v0.0.0-20170504040949-65bca51288fe
	github.com/alouca/gosnmp v0.0.0-20170620005048-04d83944c9ab
	github.com/asecurityteam/rolling/v2 v2.2.2
	github.com/bwNetFlow/ip_prefix_trie v0.0.0-20210830112018-b360b7b65c04
	github.com/cilium/ebpf v0.20.0
	github.com/dustin/go-humanize v1.0.1
	github.com/elastic/go-lumber v0.1.1
	github.com/go-co-op/gocron/v2 v2.18.0
	github.com/google/gopacket v1.1.19
	github.com/google/uuid v1.6.0
	github.com/influxdata/influxdb-client-go/v2 v2.14.0
	github.com/klauspost/compress v1.18.1
	github.com/mattn/go-sqlite3 v1.14.32
	github.com/netsampler/goflow2/v2 v2.2.3
	github.com/oschwald/maxminddb-golang v1.13.1
	github.com/osrg/gobgp/v4 v4.0.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/prometheus/client_golang v1.23.2
	github.com/rs/dnscache v0.0.0-20230804202142-fc85eb664529
	github.com/rs/zerolog v1.34.0
	github.com/viktb/asnlookup v0.1.2
	go.mongodb.org/mongo-driver v1.17.6
	golang.org/x/sys v0.38.0
	golang.org/x/text v0.31.0
	google.golang.org/protobuf v1.36.10
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/ClickHouse/ch-go v0.69.0 // indirect
	github.com/Shopify/sarama v1.38.1 // indirect
	github.com/alecthomas/participle/v2 v2.1.4 // indirect
	github.com/alouca/gologger v0.0.0-20120904114645-7d4b7291de9c // indirect
	github.com/andybalholm/brotli v1.2.0 // indirect
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgryski/go-farm v0.0.0-20240924180020-3414d57e47da // indirect
	github.com/eapache/channels v1.1.0 // indirect
	github.com/eapache/go-resiliency v1.7.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20230731223053-c322873962e3 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-faster/city v1.0.1 // indirect
	github.com/go-faster/errors v0.7.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/influxdata/line-protocol v0.0.0-20210922203350-b1ad95c89adf // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.4 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/k-sone/critbitgo v1.4.0 // indirect
	github.com/libp2p/go-reuseport v0.4.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/montanaflynn/stats v0.7.1 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/oapi-codegen/runtime v1.1.2 // indirect
	github.com/orcaman/concurrent-map/v2 v2.0.1 // indirect
	github.com/paulmach/orb v0.12.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.2 // indirect
	github.com/prometheus/procfs v0.19.2 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20250401214520-65e299d6c5c9 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/sagikazarmark/locafero v0.12.0 // indirect
	github.com/segmentio/asm v1.2.1 // indirect
	github.com/segmentio/fasthash v1.0.3 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/spf13/viper v1.21.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/viktb/go-mrt v0.0.0-20230515165434-0ce2ad0d8984 // indirect
	github.com/vishvananda/netlink v1.3.1 // indirect
	github.com/vishvananda/netns v0.0.5 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.44.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251111163417-95abcf5c77ba // indirect
	google.golang.org/grpc v1.76.0 // indirect
)

replace github.com/kaorimatz/go-mrt => github.com/TheFireMike/go-mrt v0.0.0-20220205210421-b3040c1c0b7e

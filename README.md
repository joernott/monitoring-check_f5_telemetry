# monitoring-check_f5_telemetry

This is a simple Icinga/Nagios check reading Statistics generated by the f5_telemetry module
(see <https://clouddocs.f5.com/products/extensions/f5-telemetry-streaming/latest/>)

## Usage

As this check uses viper and cobra for commandline, environment and configuration parsing, all commandline flags can also be provided by configuration file, which must then be specified with the *config* parameter or via environment variable. The variables use the prefix "CF5_" and are upper case letters. So, instead of providing the password at the command line (which is then visible in the process list), you can provide it by setting the environment variable "CF5_PASSWORD" or putting it into the config file. Choose your poison.

### Without command

Calling check_f5_telemetry without any command verb will output the help page. You need to provide one of the available commands to get soemthing useful done.

### Monitoring pool health

Using the subcommand "pool", you can monitor the pool health based on the telemetry data stored in elasticsearch.

#### Usage

```bash
  check_f5_telemetry pool [flags]

Flags:
  -h, --help              help for pool
  -i, --ignore_disabled   Ignore disabled members
  -O, --pool string       Name of the pool object to check

Global Flags:
  -A, --age_critical string   Critical if data is older than this (default "15m")
  -a, --age_warning string    Warn if data is older than this (default "5m")
  -c, --config string         Configuration file
  -C, --critical string       Critical range
  -H, --host string           Hostname of the server (default "localhost")
  -I, --index string          Name of the index containing the f5 telemetry data (default "f5_telemetry")
  -L, --logfile string        Log file (use - to log to stdout) (default "/var/log/icinga2/check_f5_telemetry.log")
  -l, --loglevel string       Log level (default "WARN")
  -p, --password string       Password for the Elasticsearch user (consider using the env variable CLE_PASSWORD instead of passing it via commandline)
  -P, --port int              Network port (default 9200)
  -y, --proxy string          Proxy (defaults to none)
  -Y, --socks                 This is a SOCKS proxy
  -s, --ssl                   Use SSL (default true)
  -T, --timeout string        Timeout understood by time.ParseDuration (default "2m")
  -u, --user string           Username for Elasticsearch
  -v, --validatessl           Validate SSL certificate (default true)
  -W, --warning string        Warning range
```

A manual call to show the health of the "kibana" pool would look like this:

```bash
read -p "Elasticcsearch User: " USER
read -s -p "Password: " CF5_PASSWORD
/usr/lib64/nagios/plugins/check_f5_telemetry pool -H "elasticsearch.example.com" -u "$USER" -O "/Common/elasticsearch-pool" -W 1 -C 2 -a 5m -A 15m
```

### Monitoring throughput
                           
Using the subcommand "throughput", you can monitor the pool health based on the telemetry data stored in elasticsearch.

#### Usage

```bash
  check_f5_telemetry throughput [flags]

Flags:
  -h, --help   help for throughput

Global Flags:
  -A, --age_critical string   Critical if data is older than this (default "15m")
  -a, --age_warning string    Warn if data is older than this (default "5m")
  -c, --config string         Configuration file
  -C, --critical string       Critical range
  -H, --host string           Hostname of the server (default "localhost")
  -I, --index string          Name of the index containing the f5 telemetry data (default "f5_telemetry")
  -L, --logfile string        Log file (use - to log to stdout) (default "/var/log/icinga2/check_f5_telemetry.log")
  -l, --loglevel string       Log level (default "WARN")
  -p, --password string       Password for the Elasticsearch user (consider using the env variable CLE_PASSWORD instead of passing it via commandline)
  -P, --port int              Network port (default 9200)
  -y, --proxy string          Proxy (defaults to none)
  -Y, --socks                 This is a SOCKS proxy
  -s, --ssl                   Use SSL (default true)
  -T, --timeout string        Timeout understood by time.ParseDuration (default "2m")
  -u, --user string           Username for Elasticsearch
  -v, --validatessl           Validate SSL certificate (default true)
  -W, --warning string        Warning range
```

A manual call to show the health of the "kibana" pool would look like this:

```bash
read -p "Elasticcsearch User: " USER
read -s -p "Password: " CF5_PASSWORD
/usr/lib64/nagios/plugins/monitoring-check_f5_telemetry\check_f5_telemetry>check_f5_telemetry throughput  -H "elasticsearch.example.com" -u "$USER" -W 20.000.000 -C 24.000.000 -a 5m -A 15m
```

## Installation

There are a whole lot of things to set up before you can use this to monitor the F5 loadbalancer. This is only a very brief overview on how to set up all involved components.

### Configure elasticsearch

Elasticsearch needs a few things in place before we can actually store data in it:

1. Create a user and role granting permissions to write data into the datastream

    ```json
    POST /_security/role/monitor_f5_telemetry
    {
      "cluster": ["monitor","read_ccr","read_ilm",  "read_slm"], 
      "indices": [
        {
          "names": "f5_telemetry",
          "privileges": ["monitor", "read",   "read_cross_cluster", "view_index_metadata"]
        }
      ]
    }

    POST /_security/user/monitor_f5_telemetry
    {
      "full_name": "Monitoring user for telemetry",
      "password": "insert_secure_password_here",
      "roles": ["monitor_f5_telemetry"]
    }
    ```

1. Create a pipeline which copies the system.systemTimestamp into @timestamp as elasticsearch no longer supports specifying the name for a
   field.

    ```json
    PUT _ingest/pipeline/f5_telemetry_add_timestamp
    {
      "description": "Add timestamp as f5 metrics does not populate that field",
      "processors": [
        {
          "set": {
            "field": "@timestamp",
            "copy_from": "system.systemTimestamp"
          }
        }
      ]
    }
    ```

1. Create a retention policy

    ```json
    PUT _ilm/policy/f5_telemetry_retention30rollover
    {
      "policy": {
        "phases": {
          "hot": {
            "min_age": "0ms",
            "actions": {
              "rollover": {
                "max_primary_shard_size": "1pb",
                "max_age": "1d"
              }
            }
          },
          "delete": {
            "min_age": "30d",
            "actions": {
              "delete": {
                "delete_searchable_snapshot": true
              }
            }
          }
        }
      }
    }
    ```

1. Create component template(s) which define the settings and mappings for the index as well as using the pipeline on every incoming record

    ```json
    PUT _component_template/component_template_f5_telemetry
    {
      "template": {
        "settings": {
          "index": {
            "lifecycle": {
              "name": "f5_telemetry_retention30rollover"
            },
            "codec": "best_compression",
            "routing": {
              "allocation": {
                "include": {
                  "datacenter": "firecell1,firecell2"
                }
              }
            },
            "mapping": {
              "total_fields": {
                "limit": "3000"
              }
            },
            "number_of_shards": "1",
            "default_pipeline": "f5_telemetry_add_timestamp",
            "number_of_replicas": "2"
          }
        }, 
        "mappings": {
          "_routing": {
            "required": false
          },
          "numeric_detection": false,
          "dynamic_date_formats": [
            "strict_date_optional_time",
            "yyyy/MM/dd HH:mm:ss Z||yyyy/MM/dd Z"
          ],
          "dynamic": true,
          "_source": {
            "excludes": [],
            "includes": [],
            "enabled": false
          },
          "date_detection": true,
          "properties": {
            "system.systemTimestamp": {
              "type": "date"
            },
            "@timestamp": {
              "type": "date"
            }
          }
        }
      }
    }
    ```

1. Create a composite template with "datastream enabled", tying the component templates together

    ```json
    PUT _index_template/f5_statistics
    {
      "index_patterns": ["f5_statistics"],
      "priority": 1000,
      "composed_of": "component_template_f5_telemetry",
      "data_stream": {
        "hidden": false
      }
    }
    ```

### Configure the loadbalancer

1. Install the f5_telemetry iApp RPM
1. Configure the loadbalancer to send the telemetry data to elasticsearch

### Install the check

1. Install the provided RPM or copy the binary to /usr/lib64/nagios/plugins

#### Configure Icinga

1. Create checkcommands for both "pool" and "throughput"

    ```icinga2
    object CheckCommand "check_f5_telemetry_pool" {
      import "plugin-check-command"

      command = [ PluginDir + "/check_f5_telemetry", "pool", ]
      env = {
        CF5_PASSWORD = "$cf5_password$"
      }
      arguments = {
        "-A" = {
          order = 1
          required = false
          value = "$cf5_age_critical$"
        }
        "-a" = {
          order = 2
          required = false
          value = "$cf5_age_warning$"
        }
        "-c" = {
          order = 3
          required = false
          value = "$cf5_config$"
        }
        "-C" = {
          order = 4
          required = false
          value = "$cf5_critical$"
        }
        "-W" = {
          order = 5
          required = false
          value = "$cf5_warning$"
        }
        "-H" = {
          order = 6
          required = false
          value = "$cf5_host$"
        }
        "-P" = {
          order = 7
          required = false
          value = "$cf5_port$"
        }
        "--ssl=false" = {
          order = 8
          required = false
          set_if = "$cf5_no_ssl$"
        }
        "--validatessl=false" = {
          order = 9
          required = false
          set_if = "$cf5_no_validate_ssl$"
        }
        "-y" = {
          order = 10
          required = false
          value = "$cf5_proxy$"
        }
        "-Y" = {
          order = 11
          required = false
          set_if = "$cf5_is_socks_proxy$"
        }
        "-u" = {
          order = 12
          required = true
          value = "$cf5_user$"
        }
        "-I" = {
          order = 13
          required = false
          value = "$cf5_index$"
        }
        "-L" = {
          order = 14
          required = false
          value = "$cf5_log_file$"
        }
        "-l" = {
          order = 15
          required = false
          value = "$cf5_log_level$"
        }
        "-T" = {
          order = 16
          required = false
          value = "$cf5_timeout$"
        }
        "-O" = {
          order = 17
          required = false
          value = "$cf5_pool$"
        }
        "--ignore_disabled=true" = {
          order = 18
          required = false
          set_if = "$cf5_ignore_disabled$"
        }
      }
    }

    object CheckCommand "check_f5_telemetry_throughput" {
      import "plugin-check-command"

      command = [ PluginDir + "/check_f5_telemetry", "throughput", ]
      env = {
        CF5_PASSWORD = "$cf5_password$"
      }
      arguments = {
        "-A" = {
          order = 1
          required = false
          value = "$cf5_age_critical$"
        }
        "-a" = {
          order = 2
          required = false
          value = "$cf5_age_warning$"
        }
        "-c" = {
          order = 3
          required = false
          value = "$cf5_config$"
        }
        "-C" = {
          order = 4
          required = false
          value = "$cf5_critical$"
        }
        "-W" = {
          order = 5
          required = false
          value = "$cf5_warning$"
        }
        "-H" = {
          order = 6
          required = false
          value = "$cf5_host$"
        }
        "-P" = {
          order = 7
          required = false
          value = "$cf5_port$"
        }
        "--ssl=false" = {
          order = 8
          required = false
          set_if = "$cf5_no_ssl$"
        }
        "--validatessl=false" = {
          order = 9
          required = false
          set_if = "$cf5_no_validate_ssl$"
        }
        "-y" = {
          order = 10
          required = false
          value = "$cf5_proxy$"
        }
        "-Y" = {
          order = 11
          required = false
          set_if = "$cf5_is_socks_proxy$"
        }
        "-u" = {
          order = 12
          required = true
          value = "$cf5_user$"
        }
        "-I" = {
          order = 13
          required = false
          value = "$cf5_index$"
        }
        "-L" = {
          order = 14
          required = false
          value = "$cf5_log_file$"
        }
        "-l" = {
          order = 15
          required = false
          value = "$cf5_log_level$"
        }
        "-T" = {
          order = 16
          required = false
          value = "$cf5_timeout$"
        }
      }
    }
    ```

1. Create service templates

    ```icinga2
    template Service "st_f5_telemetry_pool" {
      import "defaults"
      check_command = "check_f5_telemetry_pool"
      vars.cf5_host = "$host.vars.cf5_host$"
      vars.cf5_age_critical = 15m
      vars.cf5_age_warning = 5m
      vars.cf5_user = "$host.vars.cf5_user$"
      vars.cf5_password = "$host.vars.cf5_password$"
    }

    template Service "st_f5_telemetry_throughput" {
      import "defaults"
      check_command = "check_f5_telemetry_throughput"
      vars.cf5_host = "$host.vars.cf5_host$"
      vars.cf5_age_critical = 15m
      vars.cf5_age_warning = 5m
      vars.cf5_critical = "$host.vars.cf5_throughput_critical$"
      vars.cf5_warning = "$host.vars.cf5_throughput_warning$"
      vars.cf5_user = "$host.vars.cf5_user$"
      vars.cf5_password = "$host.vars.cf5_password$"
    }
    ```

1. Define checks and hosts to run them on

    ```icinga2
    apply Service "f5_elasticsearch-pool" {
      import "st_f5_telemetry_pool"

      command_endpoint = host.vars.f5_check_from
      vars.cf5_warning = 0
      vars.cf5_critical = 1
      vars.cf5_pool = "/Common/elasticsearch-pool"
      assign where "f5_elasticsearch" in host.vars.applications
    }

    apply Service "f5_throughput" {
      import "st_f5_throughput"

      command_endpoint = host.vars.f5_check_from
      vars.f5_throughput_warn = "$host.vars.f5_throughput_warn$"
      vars.f5_throughput_crit = "$host.vars.f5_throughput_crit$"
      assign where "f5" in host.vars.applications
    }

    object Endpoint "f5.example.com" {
      host = "10.0.0.1"
    }

    object Endpoint "management.example.com" {
      host = "10.0.0.2"
    }

    object Zone "f5.example.com" {
      endpoints = [ "f5.example.com", ]
      parent = "master"
    }

    object Zone "management.example.com" {
      endpoints = [ "management.example.com", ]
      parent = "master"
    }

    object Host "f5.example.com" {
      import "ht_base_ltm"

      address = "10.0.0.1"
      groups = [ "production", "f5", ]
      vars.applications = [ "f5", "f5_elasticsearch" ]
      vars.f5_check_from = "management.example.com"
      vars.f5_throughput_crit = 25690112
      vars.f5_throughput_warn = 23592960
      vars.cf5_throughput_critical = 25690112
      vars.cf5_throughput_warning = 23592960
      vars.cf5_user = "monitor_f5_telemetry"
      vars.cf5_password = "insert_secure_password_here"
    }

    object Host "management.example.com" {
      import "ht_base_rhel7"

      address = "10.0.0.2"
      groups = [ "production", "management", ]
      vars.applications = [ "f5_proxy", "icinga_agent", ]
    }

    ```


# zstats-remote-server

This zstats remote server is paired with zstats collector to solve the need of streaming prometheus stats to cloud in real time with very little use of network bandwidth.

This pairs with zstats collector, which is a modified prometheus server as mentioned [here](https://github.com/zebrium/prometheus). zstats collector runs inside the clustomer cluster, where as zstats remote server runs in the cloud, which is outside the customer's cluster.


## Architecture overview

There are two main components of this architecture:

1. **Ztstas collector**: Which is a [modified prometheus server](https://github.com/zebrium/prometheus).
2. **Zstats remote server**: This is the [remote server](https://github.com/zebrium/prometheus-remote-server), which will receive all the stats from Zstats collector.  

![](https://github.com/zebrium/ze-images/blob/master/stats_architecture.png)

This project is about Zstats remote server. For Zstats collector see https://github.com/zebrium/prometheus.

#### Architecture of this zstats remote server

##### Basic work flow:
1. Listens on an address (IP address and port) for incoming HTTP requests from Zstats collector.
2. Recreates one blob of prometheus stats for each scraped interval from every target.
3. For each blob, that it recreates, converts them into plain text files. The data in this text file will be same as the data exported from the prometheus target for one scraped interval.
4. These plain text files are stored locally for further processing, which can be notified through a GRPC interface.

![](https://github.com/zebrium/ze-images/blob/master/zstats_remote_server.png)


## Install

### Building from source

You can clone the repository yourself and build using the instructions mentioned below:

    $ mkdir -p ${GOPATH}/src/zebrium.com/stserver
    $ cd ${GOPATH}/src/zebrium.com/stserver
    $ git clone https://github.com/zebrium/prometheus-remote-server.git .
    $ go build -o ${GOPATH}/src/zebrium.com/stserver/stserver_svc_bin stats/server/main.go 

The above will generate a binary named ${GOPATH}/src/zebrium.com/stserver/stserver_svc_bin. This binary takes config file that has inputs that can be customized. Sample config file is at ${GOPATH}/src/zebrium.com/stserver/examples/config/stats.json.txt. Some notable configs of interest are:
* stats.reqs_dir: Directory, where to store the prometheus plain stats text files.
* stats.http_port: Port to listen for incoming HTTP requests.
* stats.https_enabled: Is HTTPS enabled?

One can run this server as follows:

    $ ZEBRIUM_STATS_CONFIG_FILE_LOC=${GOPATH}/src/zebrium.com/stserver/examples/config/stats.json.txt ${GOPATH}/src/zebrium.com/stserver/stserver_svc_bin 
This will run the zstats remote server, listens for HTTP posts at url: http://127.0.0.1:9905/api/v1/zstats, and by default stores the stat files at /tmp/stats/reqs directory.

Once this server is running, run your zstats collector pointing to this url. Running the zstats collector is documented [here](https://github.com/zebrium/prometheus/blob/release-2.14/README.md) look for Install section.

Here is the sample output of /tmp/stats/reqs directory after receiving some scraped stats:
```
$ ls  /tmp/stats/reqs/Zebrium/192.168.120.51_9100.prometheus
1584152555489133
1584152565506484
$
```
One can see that we have received two scrapes of data from Prometheus target "192.168.120.51_9100.prometheus". It creates one directory for each scrape, and the name of that directory is epoch when it was received by zstats remote server. Looking at the contents of one scraped interval's data from one target:
```
$ ls  /tmp/stats/reqs/Zebrium/192.168.120.51_9100.prometheus/1584152555489133
1584152555489133.data.gz  1584152555489133.json
```
As you can see, it has two files: json and data.gz file. 

***json file***: has the metadata and labels that are common for all the samples in this scrape.
```
$ python3 -m json.tool < /tmp/stats/reqs/Zebrium/192.168.120.51_9100.prometheus/1584152555489133/1584152555489133.json 
{
    "account": "Zebrium",
    "type": "prometheus",
    "iid": "192.168.120.51:9100.prometheus",
    "is_compressed": true,
    "kvs": [
        {
            "key": "zid_host",
            "value": "anil-host1"
        },
        {
            "key": "zid_pod_name",
            "value": "anil-pod1"
        },
        {
            "key": "instance",
            "value": "192.168.120.51:9100"
        },
        {
            "key": "container_name",
            "value": "anil-container1"
        },
        {
            "key": "host",
            "value": "anil-host1"
        },
        {
            "key": "job",
            "value": "prometheus"
        },
        {
            "key": "zid_container_name",
            "value": "anil"
        }
    ],
    "dts": 1584152545363,
    "olabels": [
        "instance",
        "job",
    ]
}
```
This json file contains : 
* account: Is the name of the customer.
* type: prometheus.
* iid: Combination of instance and job from prometheus label values. unique instance id.
* kvs: Array of labels that are same for all samples in data.gz file. They are all put in here to save the space in data.gz file.
* dts: default time stamp in epoch. This is the default timestamp to use, if there is not timestamp inside a sample of data.gz file.
* olabels : there are optional labels, that can be safely ignored for ML/AI purpose.

*data.gz* file:
```
zcat /tmp/stats/reqs/Zebrium/192.168.120.51_9100.prometheus/1584152555489133/1584152555489133.data.gz | head -15
# HELP go_gc_duration_seconds A summary of the GC invocation durations.
# TYPE go_gc_duration_seconds summary
go_gc_duration_seconds{quantile="0"} 0.000006
go_gc_duration_seconds{quantile="0.25"} 0.000019
go_gc_duration_seconds{quantile="0.5"} 0.000026
go_gc_duration_seconds{quantile="0.75"} 0.000041
go_gc_duration_seconds{quantile="1"} 0.000138
go_gc_duration_seconds_sum{} 0.103264
go_gc_duration_seconds_count{} 3046.000000
# HELP go_goroutines Number of goroutines that currently exist.
# TYPE go_goroutines gauge
go_goroutines{} 6.000000
# HELP go_info Information about the Go environment.
# TYPE go_info gauge
go_info{version="go1.12.5"} 1.000000
```
This data.gz file contains samples as if we scraped the data from the target locally. It has HELP, TYPE, and stats sample information.


## More information

  * Link to zebrium Blog: TODO 

## License

Apache License 2.0, see [LICENSE](https://github.com/prometheus/prometheus/blob/master/LICENSE).

## Contributors
* Anil Nanduri (Zebrium)
* Dara Hazeghi (Zebrium)
* Brady Zuo (Zebrium)


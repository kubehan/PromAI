-- Generated from config.yaml metric_types
BEGIN;

INSERT INTO metric_types (type_name, sort_order, created_at, updated_at)
SELECT 'L1-基础设施层：硬件设备监控', 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM metric_types WHERE type_name = 'L1-基础设施层：硬件设备监控'
);

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'CPU性能状态监控', 'CPU性能状态监控-1h内平均使用率', 'avg_over_time(
  (
    100 - avg by(instance, nodename) (irate(node_cpu_seconds_total{mode=''idle''}[5m]) * 100)
  )[1h:]
) * on(instance) group_left(nodename) node_uname_info',
  99, 'greater', '', '%', '{"instance":"实例地址"}',
  0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L1-基础设施层：硬件设备监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'CPU性能状态监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, '机器负载与CPU核心数比率监控', '', '(node_load5/count without (cpu, mode) (node_cpu_seconds_total{mode="system"}))* on(instance) group_left(nodename) (node_uname_info{nodename !~"hadoop.*"})',
  3, 'greater', '', '倍', '{"instance":"实例地址","nodename":"主机名"}',
  1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L1-基础设施层：硬件设备监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = '机器负载与CPU核心数比率监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, '内存性能状态监控', '', 'avg_over_time((100 - ((node_memory_MemAvailable_bytes * 100)/node_memory_MemTotal_bytes))[1h:]) * on(instance) group_left(nodename) node_uname_info',
  95, 'greater', '', '%', '{"instance":"实例地址"}',
  2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L1-基础设施层：硬件设备监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = '内存性能状态监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, '存储设备状态监控', '', '(
  (
    (100 - (node_filesystem_avail_bytes * 100 / node_filesystem_size_bytes{mountpoint=~"/home|/mnt.*|/data.*|/opt.*"}))
    and
    ON (instance, device, mountpoint) node_filesystem_readonly == 0
  )
  * ON (instance) group_left(nodename) node_uname_info
)',
  90, 'greater', 'critical', '%', '{"device":"磁盘设备","instance":"实例地址","mountpoint":"盘路径","nodename":"主机名"}',
  3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L1-基础设施层：硬件设备监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = '存储设备状态监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, '磁盘读写IO性能监控3小时内平均使用率', '磁盘读写IO性能监控-3小时内平均使用率', 'min_over_time((rate(node_disk_io_time_seconds_total{}[5m]) * 100)[3h:5m]) >= 10',
  95, 'greater', '', '%', '{"device":"磁盘设备","instance":"实例地址","service":"服务名称"}',
  4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L1-基础设施层：硬件设备监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = '磁盘读写IO性能监控3小时内平均使用率'
  );

INSERT INTO inspection_templates (name, description, created_at, updated_at)
SELECT 'L1-基础设施层：硬件设备监控', '基于「L1-基础设施层：硬件设备监控」自动创建的默认模板', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM inspection_templates WHERE name = 'L1-基础设施层：硬件设备监控'
);

INSERT OR IGNORE INTO inspection_template_metrics (template_id, metric_config_id)
SELECT t.id, mc.id
FROM inspection_templates t
JOIN metric_types mt ON mt.type_name = 'L1-基础设施层：硬件设备监控'
JOIN metric_configs mc ON mc.metric_type_id = mt.id
WHERE t.name = 'L1-基础设施层：硬件设备监控';

INSERT INTO metric_types (type_name, sort_order, created_at, updated_at)
SELECT 'L2-网络层：网络连接监控', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM metric_types WHERE type_name = 'L2-网络层：网络连接监控'
);

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, '网络丢包率监控', '', 'rate(node_network_receive_drop_total{device!~"cal.*"}[5m])',
  100, 'greater', '', '', '{"instance":"实例地址"}',
  0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L2-网络层：网络连接监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = '网络丢包率监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, '网络连接负载监控', '', 'node_netstat_Tcp_CurrEstab{}',
  30000, 'greater', 'critical', '', '{"instance":"实例地址"}',
  1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L2-网络层：网络连接监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = '网络连接负载监控'
  );

INSERT INTO inspection_templates (name, description, created_at, updated_at)
SELECT 'L2-网络层：网络连接监控', '基于「L2-网络层：网络连接监控」自动创建的默认模板', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM inspection_templates WHERE name = 'L2-网络层：网络连接监控'
);

INSERT OR IGNORE INTO inspection_template_metrics (template_id, metric_config_id)
SELECT t.id, mc.id
FROM inspection_templates t
JOIN metric_types mt ON mt.type_name = 'L2-网络层：网络连接监控'
JOIN metric_configs mc ON mc.metric_type_id = mt.id
WHERE t.name = 'L2-网络层：网络连接监控';

INSERT INTO metric_types (type_name, sort_order, created_at, updated_at)
SELECT 'L3-容器层：K8s运行环境核心服务监控', 2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM metric_types WHERE type_name = 'L3-容器层：K8s运行环境核心服务监控'
);

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'K8s Node 节点状态检查', 'Node 节点状态异常或宕机', 'kube_node_status_condition{job="kube-state-metrics",condition="Ready",status="true"}  * on(node) group_left(label_hostname) (kube_node_labels{})',
  1, 'equal', 'normal', '', '{"instance":"实例地址","node":"主机名"}',
  0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L3-容器层：K8s运行环境核心服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'K8s Node 节点状态检查'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'K8s 基础组件健康状态检查', 'K8s 基础组件健康状态检查', 'up{job=~"kube.*"}',
  1, 'equal', 'normal', '', '{"instance":"实例地址","service":"组件名称"}',
  1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L3-容器层：K8s运行环境核心服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'K8s 基础组件健康状态检查'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, '集群核心网关服务状态监控--自定义监控建设中', 'K8s集群关键服务状态统计', 'key_pod_status',
  1, 'equal', 'normal', '', '{"component":"服务名称","describe":"服务描述","hostname":"主机名称","owner":"负责人"}',
  2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L3-容器层：K8s运行环境核心服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = '集群核心网关服务状态监控--自定义监控建设中'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, '系统关键Pod运行状态监控', '系统关键Pod运行状态监控', 'kube_pod_status_ready{condition="false",namespace=~"kube-system|.*prod.*|.*infra.*|.*es.*|monitoring|kong|proxy|loki",pod !~".*backup.*"}',
  0, 'equal', 'normal', '', '{"condition":"状态","namespace":"命名空间","pod":"服务名称"}',
  3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L3-容器层：K8s运行环境核心服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = '系统关键Pod运行状态监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, '分布式存储etcd状态监控', '分布式存储etcd状态监控', 'routing_etcd_node_status',
  1, 'equal', 'normal', '', '{"component":"服务组件","describe":"描述","hostname":"主机名称","owner":"负责人","target":"线上 or 线下"}',
  4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L3-容器层：K8s运行环境核心服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = '分布式存储etcd状态监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'DNS服务响应性能监控', 'DNS服务响应性能监控', 'histogram_quantile(0.99,sum(rate(coredns_dns_request_duration_seconds_bucket{}[5m])) by(le,job,instance,pod))',
  2, 'greater', '', 's', '{"instance":"实例地址","job":"服务名","pod":"DNS服务节点"}',
  5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L3-容器层：K8s运行环境核心服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'DNS服务响应性能监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, '容器CPU资源使用监控-1小时内平均使用率', '容器CPU资源使用监控', '100 *  avg_over_time(
  (
    sum by(container, namespace, pod) (
      rate(container_cpu_usage_seconds_total{
        container!="",
        namespace !~ ".*preview.*"
      }[5m])
    )
    /
    sum by(container, namespace, pod) (
      kube_pod_container_resource_limits{
        container!="",
        resource="cpu",
        namespace !~ ".*preview.*"
      }
    )
  )[1h:]
) > 80',
  95, 'greater', '', '%', '{"namespace":"命名空间","pod":"服务名称"}',
  6, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L3-容器层：K8s运行环境核心服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = '容器CPU资源使用监控-1小时内平均使用率'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, '容器内存资源使用监控-1小时内平均使用率', '容器内存资源使用监控', '100 *  avg_over_time( (
  sum by(container, namespace, pod) (
    container_memory_working_set_bytes{
      container!="",
      container!="POD",
      namespace !~ ".*preview.*"
    }
  )
  /
  sum by(container, namespace, pod) (
    kube_pod_container_resource_limits{
      container!="",
      container!="POD",
      resource="memory",
      namespace !~ ".*preview.*"
    }
  )
)[1h:] ) > 80',
  95, 'greater', '', '%', '{"namespace":"命名空间","pod":"服务名称"}',
  7, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L3-容器层：K8s运行环境核心服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = '容器内存资源使用监控-1小时内平均使用率'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, '接入层路由服务健康监控', '接入层路由服务健康监控', 'haproxy_up{service="routing-exporter"}',
  1, 'equal', 'normal', '', '{"hostname":"主机名称","instance":"实例地址","pod":"服务名称"}',
  8, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L3-容器层：K8s运行环境核心服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = '接入层路由服务健康监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, '节点Pod资源数量使用负载监控', '节点Pod资源使用负载监控', 'count by(cluster, node) (
  (kube_pod_status_phase{job="kube-state-metrics", phase="Running"} == 1)
  * on(instance, pod, namespace, cluster) group_left(node)
  kube_pod_info{job="kube-state-metrics"}
) / max by(cluster, node) (
  kube_node_status_capacity{job="kube-state-metrics", resource="pods"}
) > 0',
  95, 'greater', '', '%', '{"node":"主机名"}',
  9, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L3-容器层：K8s运行环境核心服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = '节点Pod资源数量使用负载监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'P90用户中心who接口5分钟内平均响应时长大于5s 告警', 'P90用户中心who接口5分钟内平均响应时长大于5s 告警', 'histogram_quantile(
  0.9,
  sum(rate(user_who_response_latency_seconds_bucket[5m]))
    by (namespace, container, exported_endpoint, instance, le)
)',
  10, 'greater', '', '', '{"exported_endpoint":"url","namespace":"命名空间"}',
  10, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L3-容器层：K8s运行环境核心服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'P90用户中心who接口5分钟内平均响应时长大于5s 告警'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, ' P90用户中心cas登录接口5分钟内平均响应时长大于5s 告警', ' P90用户中心cas登录接口5分钟内平均响应时长大于5s 告警', 'histogram_quantile(
  0.9,
  sum(rate(cas_js_login_response_latency_seconds_bucket[5m]))
    by (namespace, container, exported_endpoint, instance, le)
)',
  10, 'greater', '', '', '{"exported_endpoint":"url","namespace":"命名空间"}',
  11, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L3-容器层：K8s运行环境核心服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = ' P90用户中心cas登录接口5分钟内平均响应时长大于5s 告警'
  );

INSERT INTO inspection_templates (name, description, created_at, updated_at)
SELECT 'L3-容器层：K8s运行环境核心服务监控', '基于「L3-容器层：K8s运行环境核心服务监控」自动创建的默认模板', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM inspection_templates WHERE name = 'L3-容器层：K8s运行环境核心服务监控'
);

INSERT OR IGNORE INTO inspection_template_metrics (template_id, metric_config_id)
SELECT t.id, mc.id
FROM inspection_templates t
JOIN metric_types mt ON mt.type_name = 'L3-容器层：K8s运行环境核心服务监控'
JOIN metric_configs mc ON mc.metric_type_id = mt.id
WHERE t.name = 'L3-容器层：K8s运行环境核心服务监控';

INSERT INTO metric_types (type_name, sort_order, created_at, updated_at)
SELECT 'L4-中间件层：MongoDB数据库监控', 3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM metric_types WHERE type_name = 'L4-中间件层：MongoDB数据库监控'
);

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'MongoDB副本集节点状态监控', 'MongoDB副本集节点状态监控', 'mongodb_replset_member_health',
  1, 'not_equal', '', '', '{"job":"采集器名称","name":"服务地址"}',
  0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：MongoDB数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'MongoDB副本集节点状态监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'MongoDB复制延迟监控', 'MongoDB副本复制延迟监控', 'mongodb_mongod_replset_member_replication_lag or mongodb_replset_member_state',
  60, 'greater', '', 's', '{"job":"采集器名称","name":"服务地址"}',
  1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：MongoDB数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'MongoDB复制延迟监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'MongoDB逻辑会话数监控', 'MongoDB逻辑会话数监控', 'mongodb_sessions_active_total OR mongodb_shard_active_sessions OR mongodb_rs_session_count',
  800000, 'greater', '', '', '{"instance":"主机地址","job":"采集器名称","shard":"分片信息"}',
  2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：MongoDB数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'MongoDB逻辑会话数监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'MongoDB连接数监控', 'MongoDB连接数使用率监控', 'avg by (instance) (mongodb_connections{state="current"}) / avg by (instance) (mongodb_connections{state="available"}) * 100',
  80, 'greater', '', '%', '{"instance":"实例地址"}',
  3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：MongoDB数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'MongoDB连接数监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'MongoDB内存使用率', 'MongoDB内存使用率监控', '(max by(container, pod, node) (container_memory_working_set_bytes{container!="",container!="POD",pod=~".*mongo.*"}) / max by(container, pod, node) (kube_pod_container_resource_limits{container!="",container!="POD",pod=~".*mongo.*",resource="memory"})) * 100',
  95, 'greater_equal', '', '%', '{"container":"实例地址","pod":"pod信息"}',
  4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：MongoDB数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'MongoDB内存使用率'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'MongoDB性能监控', 'MongoDB增删改查性能监控', 'mongodb_crud_time_ms or mongodb_operation_duration_seconds',
  3, 'greater_equal', '', '', '{"instance":"主机地址","operation":"具体操作类型","shard":"副本信息"}',
  5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：MongoDB数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'MongoDB性能监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'MongoDB网络流量', 'MongoDB网络流量监控', 'rate(mongodb_network_bytes_total[1m])',
  1e+07, 'greater', '', '', '{"instance":"实例地址","pod":"pod信息","service":"分片信息","state":"流量状态"}',
  6, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：MongoDB数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'MongoDB网络流量'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'MongoDB服务宕机', 'MongoDB服务宕机监控', 'mongodb_up',
  0, 'equal', '', '', '{"instance":"实例地址","job":"分片信息","pod":"pod信息"}',
  7, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：MongoDB数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'MongoDB服务宕机'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'MongoDB实例发生重启', 'MongoDB重启监控', 'mongodb_instance_uptime_seconds',
  60, 'less', '', '', '{"instance":"实例地址","pod":"pod信息","service":"分片信息"}',
  8, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：MongoDB数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'MongoDB实例发生重启'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'MongoDB断言告警 - 需要立即检查', 'MongoDB数据损坏风险监控', 'rate(mongodb_asserts_total{type=~"regular|message"}[5m])',
  0, 'equal', 'normal', '', '{"instance":"实例信息","pod":"pod信息","service":"分片信息","type":"状态类型"}',
  9, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：MongoDB数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'MongoDB断言告警 - 需要立即检查'
  );

INSERT INTO inspection_templates (name, description, created_at, updated_at)
SELECT 'L4-中间件层：MongoDB数据库监控', '基于「L4-中间件层：MongoDB数据库监控」自动创建的默认模板', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM inspection_templates WHERE name = 'L4-中间件层：MongoDB数据库监控'
);

INSERT OR IGNORE INTO inspection_template_metrics (template_id, metric_config_id)
SELECT t.id, mc.id
FROM inspection_templates t
JOIN metric_types mt ON mt.type_name = 'L4-中间件层：MongoDB数据库监控'
JOIN metric_configs mc ON mc.metric_type_id = mt.id
WHERE t.name = 'L4-中间件层：MongoDB数据库监控';

INSERT INTO metric_types (type_name, sort_order, created_at, updated_at)
SELECT 'L4-中间件层：MySQL数据库监控', 4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM metric_types WHERE type_name = 'L4-中间件层：MySQL数据库监控'
);

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'MySQL连接数', 'MySQL连接数使用率监控', 'avg by (instance) (mysql_global_status_threads_connected) / avg by (instance) (mysql_global_variables_max_connections) * 100',
  80, 'greater', '', '%', '{"instance":"实例地址"}',
  0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：MySQL数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'MySQL连接数'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'MySQL thread running监控', 'MySQL并发监控', 'mysql_global_status_threads_running',
  50, 'greater', '', '', '{"instance":"实例地址","service":"服务名"}',
  1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：MySQL数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'MySQL thread running监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'MySQL活跃线程异常增长', 'MySQL活跃线程异常增长监控', 'rate(mysql_global_status_threads_created[5m])',
  10, 'greater', '', '', '{"instance":"实例地址","job":"采集器名称"}',
  2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：MySQL数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'MySQL活跃线程异常增长'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'MySQL主从复制错误', 'MySQL主从复制错误监控', 'mysql_slave_status_last_io_errno',
  0, 'not_equal', '', '', '{"instance":"实例地址","job":"采集器名称","master_host":"mysql主库地址"}',
  3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：MySQL数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'MySQL主从复制错误'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'MySQL SLAVE SQL线程停止', 'MySQL SLAVE SQL线程停止监控', 'mysql_slave_status_master_server_id > 0 and ON (instance) mysql_slave_status_slave_sql_running',
  0, 'equal', '', '', '{"instance":"实例地址","job":"采集器名称","master_host":"mysql主库地址"}',
  4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：MySQL数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'MySQL SLAVE SQL线程停止'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'MySQL慢查询监控', 'MySQL慢查询数量监控', 'increase(mysql_global_status_slow_queries[1m])',
  150, 'greater', '', '', '{"instance":"实例地址","service":"服务名"}',
  5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：MySQL数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'MySQL慢查询监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'MySQL打开文件数监控', 'MySQL打开文件数监控', 'avg by (instance) (mysql_global_status_innodb_num_open_files) / avg by (instance)(mysql_global_variables_open_files_limit) * 100',
  80, 'greater', '', '%', '{"instance":"实例地址"}',
  6, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：MySQL数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'MySQL打开文件数监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'MySQL InnoDB缓冲池命中率', 'MySQL InnoDB缓冲池命中率监控', '(1 - mysql_global_status_innodb_buffer_pool_reads / mysql_global_status_innodb_buffer_pool_read_requests) * 100',
  95, 'less', 'critical', '%', '{"instance":"实例地址","job":"采集器名称"}',
  7, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：MySQL数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'MySQL InnoDB缓冲池命中率'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'MySQL临时表创建频繁', 'MySQL临时表创建频繁监控', 'rate(mysql_global_status_created_tmp_disk_tables[5m]) ',
  100, 'greater', '', '', '{"instance":"实例地址","job":"采集器名称"}',
  8, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：MySQL数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'MySQL临时表创建频繁'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'MySQL连接错误率', 'MySQL连接错误率监控', 'rate(mysql_global_status_connection_errors_total[2m])  ',
  10, 'greater', '', '', '{"instance":"实例地址","job":"采集器名称"}',
  9, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：MySQL数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'MySQL连接错误率'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'MySQL服务宕机', 'MySQL服务宕机监控', 'mysql_up',
  0, 'equal', '', '', '{"instance":"实例地址","job":"采集器名称"}',
  10, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：MySQL数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'MySQL服务宕机'
  );

INSERT INTO inspection_templates (name, description, created_at, updated_at)
SELECT 'L4-中间件层：MySQL数据库监控', '基于「L4-中间件层：MySQL数据库监控」自动创建的默认模板', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM inspection_templates WHERE name = 'L4-中间件层：MySQL数据库监控'
);

INSERT OR IGNORE INTO inspection_template_metrics (template_id, metric_config_id)
SELECT t.id, mc.id
FROM inspection_templates t
JOIN metric_types mt ON mt.type_name = 'L4-中间件层：MySQL数据库监控'
JOIN metric_configs mc ON mc.metric_type_id = mt.id
WHERE t.name = 'L4-中间件层：MySQL数据库监控';

INSERT INTO metric_types (type_name, sort_order, created_at, updated_at)
SELECT 'L4-中间件层：Redis缓存服务监控', 5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM metric_types WHERE type_name = 'L4-中间件层：Redis缓存服务监控'
);

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'Redis服务状态', 'Redis实例运行状态监控', 'redis_up{job!~".*sentinel.*"}',
  1, 'equal', 'normal', '', '{"instance":"实例地址","service":"采集器命名"}',
  0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：Redis缓存服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'Redis服务状态'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'Redis连接数', 'Redis连接数使用率监控', '(redis_connected_clients / redis_config_maxclients) * 100',
  80, 'greater', '', '%', '{"instance":"实例地址","service":"采集器命名"}',
  1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：Redis缓存服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'Redis连接数'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'Redis内存使用', 'Redis内存使用率监控', 'redis_memory_used_bytes / redis_memory_max_bytes',
  0.8, 'greater', '', '%', '{"instance":"实例地址","service":"采集器命名"}',
  2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：Redis缓存服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'Redis内存使用'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'Redis内存碎片', 'Redis内存碎片率监控', 'redis_memory_used_rss_bytes/redis_memory_used_bytes',
  2.5, 'greater', '', '', '{"instance":"实例地址","service":"采集器命名"}',
  3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：Redis缓存服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'Redis内存碎片'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'Redis慢查询', 'Redis慢查询数量监控', 'rate(redis_slowlog_length[5m])',
  50, 'greater', '', '', '{"instance":"实例地址","service":"采集器命名"}',
  4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：Redis缓存服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'Redis慢查询'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'Redis主从延迟', 'Redis主从同步延迟监控', 'redis_connected_slave_lag_seconds',
  10, 'greater', '', '', '{"instance":"实例地址","service":"采集器命名","slave_ip":"服务地址","slave_port":"服务端口"}',
  5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：Redis缓存服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'Redis主从延迟'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'Redis主从切换', 'Redis主从角色切换监控', 'changes(redis_instance_info{role="master"}[1m])',
  0, 'greater', 'critical', '', '{"instance":"实例地址","redis_version":"redis版本号","service":"采集器命名"}',
  6, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：Redis缓存服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'Redis主从切换'
  );

INSERT INTO inspection_templates (name, description, created_at, updated_at)
SELECT 'L4-中间件层：Redis缓存服务监控', '基于「L4-中间件层：Redis缓存服务监控」自动创建的默认模板', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM inspection_templates WHERE name = 'L4-中间件层：Redis缓存服务监控'
);

INSERT OR IGNORE INTO inspection_template_metrics (template_id, metric_config_id)
SELECT t.id, mc.id
FROM inspection_templates t
JOIN metric_types mt ON mt.type_name = 'L4-中间件层：Redis缓存服务监控'
JOIN metric_configs mc ON mc.metric_type_id = mt.id
WHERE t.name = 'L4-中间件层：Redis缓存服务监控';

INSERT INTO metric_types (type_name, sort_order, created_at, updated_at)
SELECT 'L4-中间件层：PostgreSQL数据库监控', 6, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM metric_types WHERE type_name = 'L4-中间件层：PostgreSQL数据库监控'
);

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'PostgreSQL服务状态监控-建设中', 'PostgreSQL服务运行状态监控', 'absent(up{job=~"pgsql"})',
  1, 'equal', '', '', '{"instance":"实例地址","job":"采集器名称"}',
  0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：PostgreSQL数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'PostgreSQL服务状态监控-建设中'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'PostgreSQL连接数监控', 'PostgreSQL连接数负载监控', 'pg_stat_activity_count',
  1500, 'greater_equal', '', '', '{"instance":"实例地址","job":"采集器名称","server":"服务地址","state":"state"}',
  1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：PostgreSQL数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'PostgreSQL连接数监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'PostgreSQL主从延迟监控', 'PostgreSQL主从同步延迟监控', 'pg_replication_lag_seconds',
  60, 'greater', '', '', '{"instance":"实例地址","job":"采集器名称","pod":"pod命名"}',
  2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：PostgreSQL数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'PostgreSQL主从延迟监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'PostgreSQL长事务监控', 'PostgreSQL长事务数监控', 'pg_up',
  1, 'greater', '', '', '{"instance":"实例地址","job":"采集器名称","pod":"pod命名","server":"服务地址","state":"state"}',
  3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：PostgreSQL数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'PostgreSQL长事务监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'PostgreSQL死锁监控', 'PostgreSQL数据库死锁监控', 'rate(pg_stat_database_deadlocks[30m])',
  5, 'greater', '', '', '{"instance":"实例地址","job":"采集器名称","pod":"pod命名"}',
  4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L4-中间件层：PostgreSQL数据库监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'PostgreSQL死锁监控'
  );

INSERT INTO inspection_templates (name, description, created_at, updated_at)
SELECT 'L4-中间件层：PostgreSQL数据库监控', '基于「L4-中间件层：PostgreSQL数据库监控」自动创建的默认模板', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM inspection_templates WHERE name = 'L4-中间件层：PostgreSQL数据库监控'
);

INSERT OR IGNORE INTO inspection_template_metrics (template_id, metric_config_id)
SELECT t.id, mc.id
FROM inspection_templates t
JOIN metric_types mt ON mt.type_name = 'L4-中间件层：PostgreSQL数据库监控'
JOIN metric_configs mc ON mc.metric_type_id = mt.id
WHERE t.name = 'L4-中间件层：PostgreSQL数据库监控';

INSERT INTO metric_types (type_name, sort_order, created_at, updated_at)
SELECT 'L5-接入层：API网关代理监控', 7, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM metric_types WHERE type_name = 'L5-接入层：API网关代理监控'
);

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, '接入层apirouter状态码监控', '接入层apirouter 5xx状态码监控', 'sum(rate(kong_http_status{job="apirouter-exporter",code=~"5..", exported_service!~".*preview.*"}[10m])) by (exported_service) / sum(rate(kong_http_status{job="apirouter-exporter"}[10m])) by (exported_service)',
  5, 'greater_equal', '', '', '{"exported_service":"服务名称"}',
  0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L5-接入层：API网关代理监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = '接入层apirouter状态码监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'Routing路由状态监控', 'Routing服务健康状态监控', 'haproxy_up',
  1, 'equal', 'normal', '', '{"instance":"主机地址","namespace":"命名空间","pod":"服务名称"}',
  1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L5-接入层：API网关代理监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'Routing路由状态监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, '接入层Routing后端服务5分钟内平均响应时间', '接入层Routing后端服务5分钟内平均响应时间', 'avg_over_time(haproxy_server_http_response_time_average_seconds{backend !~"modtunnel|sreldap|databus-prod-17", namespace="marathon-prod"}[10m]) > 0.001',
  10, 'greater_equal', '', 's', '{"backend":"后端服务名称","hostname":"主机地址","namespace":"命名空间","server":"服务名称"}',
  2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L5-接入层：API网关代理监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = '接入层Routing后端服务5分钟内平均响应时间'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, '接入层5xx异常状态码监控', '接入层状态码监控', '(sum(rate(nginx_ingress_controller_requests{status=~"5..",namespace!~".*preview.*"}[10m])) by (ingress, exported_service) / sum(rate(nginx_ingress_controller_requests{namespace!~".*preview.*"}[10m])) by (ingress, exported_service) ) * 100 >= 0',
  5, 'greater_equal', '', '', '{"exported_service":"服务名称","ingress":"ingress名称"}',
  3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L5-接入层：API网关代理监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = '接入层5xx异常状态码监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'Web服务状态监控', 'Nginx Web服务状态监控-自定义监控建设中', 'network_port_status{process="nginx"}',
  1, 'equal', 'normal', '', '{"instance":"主机地址","port":"80","process":"服务名称","src_host":"主机名"}',
  4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L5-接入层：API网关代理监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'Web服务状态监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'OpenAPI服务状态监控', 'OpenAPI接口服务状态监控-自定义监控建设中', 'openapi_status{target="prod",component="openapi"}',
  1, 'equal', 'normal', '', '{"instance":"实例地址"}',
  5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L5-接入层：API网关代理监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'OpenAPI服务状态监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'LDAP服务状态监控', 'LDAP认证服务状态监控-自定义监控建设中', 'ldap_server_status{hostname=~"web1.*"}',
  1, 'equal', 'normal', '', '{"component":"服务名","hostname":"主机名","instance":"实例地址"}',
  6, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L5-接入层：API网关代理监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'LDAP服务状态监控'
  );

INSERT INTO inspection_templates (name, description, created_at, updated_at)
SELECT 'L5-接入层：API网关代理监控', '基于「L5-接入层：API网关代理监控」自动创建的默认模板', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM inspection_templates WHERE name = 'L5-接入层：API网关代理监控'
);

INSERT OR IGNORE INTO inspection_template_metrics (template_id, metric_config_id)
SELECT t.id, mc.id
FROM inspection_templates t
JOIN metric_types mt ON mt.type_name = 'L5-接入层：API网关代理监控'
JOIN metric_configs mc ON mc.metric_type_id = mt.id
WHERE t.name = 'L5-接入层：API网关代理监控';

INSERT INTO metric_types (type_name, sort_order, created_at, updated_at)
SELECT 'L6-应用层：业务服务监控', 8, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM metric_types WHERE type_name = 'L6-应用层：业务服务监控'
);

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, '文件上传服务监控', '文件上传服务状态监控-建设中', 'itmp_coreapi_status',
  1, 'equal', 'normal', '', '{"component":"服务名","hostname":"主机名","instance":"地址","owner":"负责人"}',
  0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L6-应用层：业务服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = '文件上传服务监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, 'CMDB服务状态监控', 'CMDB服务状态监控-自定义监控建设中', 'cmdb_status',
  1, 'equal', 'normal', '', '{"component":"服务名","hostname":"主机名","instance":"地址","owner":"负责人"}',
  1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L6-应用层：业务服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = 'CMDB服务状态监控'
  );

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, '业务核心服务监控采集检查', '核心服务监控覆盖检查', 'up{job=~".*eventlog.*|.*passport.*|.*ucenter.*",namespace!~".*preview.*|.*ingress.*"}',
  1, 'equal', 'normal', '', '{"instance":"地址","job":"服务对应exporter命名","namespace":"命名空间","service":"服务名"}',
  2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L6-应用层：业务服务监控'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = '业务核心服务监控采集检查'
  );

INSERT INTO inspection_templates (name, description, created_at, updated_at)
SELECT 'L6-应用层：业务服务监控', '基于「L6-应用层：业务服务监控」自动创建的默认模板', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM inspection_templates WHERE name = 'L6-应用层：业务服务监控'
);

INSERT OR IGNORE INTO inspection_template_metrics (template_id, metric_config_id)
SELECT t.id, mc.id
FROM inspection_templates t
JOIN metric_types mt ON mt.type_name = 'L6-应用层：业务服务监控'
JOIN metric_configs mc ON mc.metric_type_id = mt.id
WHERE t.name = 'L6-应用层：业务服务监控';

INSERT INTO metric_types (type_name, sort_order, created_at, updated_at)
SELECT 'L7-采集层：监控数据采集状态', 9, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM metric_types WHERE type_name = 'L7-采集层：监控数据采集状态'
);

INSERT INTO metric_configs (
  metric_type_id, datasource_id, name, description, query,
  threshold, threshold_type, threshold_status, unit, labels_json,
  sort_order, created_at, updated_at
)
SELECT mt.id, NULL, '监控数据采集状态监控', 'Exporter数据采集服务状态监控', 'up{job!~"hive.*|hadoop.*|hbase.*|spark.*|yarn.*|.*kafka.*"} ',
  1, 'equal', 'normal', '', '{"instance":"地址","job":"服务对应exporter命名","service":"服务名"}',
  0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM metric_types mt
WHERE mt.type_name = 'L7-采集层：监控数据采集状态'
  AND NOT EXISTS (
    SELECT 1 FROM metric_configs mc
    WHERE mc.metric_type_id = mt.id
      AND mc.datasource_id IS NULL
      AND mc.name = '监控数据采集状态监控'
  );

INSERT INTO inspection_templates (name, description, created_at, updated_at)
SELECT 'L7-采集层：监控数据采集状态', '基于「L7-采集层：监控数据采集状态」自动创建的默认模板', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM inspection_templates WHERE name = 'L7-采集层：监控数据采集状态'
);

INSERT OR IGNORE INTO inspection_template_metrics (template_id, metric_config_id)
SELECT t.id, mc.id
FROM inspection_templates t
JOIN metric_types mt ON mt.type_name = 'L7-采集层：监控数据采集状态'
JOIN metric_configs mc ON mc.metric_type_id = mt.id
WHERE t.name = 'L7-采集层：监控数据采集状态';

INSERT INTO inspection_templates (name, description, created_at, updated_at)
SELECT '全局模板', '包含全部全局指标的默认模板', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
  SELECT 1 FROM inspection_templates WHERE name = '全局模板'
);

INSERT OR IGNORE INTO inspection_template_metrics (template_id, metric_config_id)
SELECT t.id, mc.id
FROM inspection_templates t
JOIN metric_configs mc ON mc.metric_type_id IS NOT NULL
WHERE t.name = '全局模板';

COMMIT;

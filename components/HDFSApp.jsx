import React, { Component, PropTypes } from 'react';
import ApplicationForm from './ApplicationForm';

const configList = [
  {
    type: '基础服务',
    scale: 0,
    replicas: 1,
    cpu: '1',
    memory: '2G',
  },
  {
    type: '专业服务',
    scale: 1,
    replicas: 3,
    cpu: '2',
    memory: '4G',
  },
  {
    type: '专业服务',
    scale: 2,
    replicas: 6,
    cpu: '2',
    memory: '4G',
  },
  {
    type: '专业服务',
    scale: 4,
    replicas: 12,
    cpu: '2',
    memory: '4G',
  },
];

const getNameNode = (name) => ({
  metadata: {
    name: `hadoop-namenode-${name}`,
  },
  spec: {
    replicas: 0,
    template: {
      metadata: {
        annotations: {
          'config/core-site.xml': `
<configuration>
  <property>
    <name>hadoop.tmp.dir</name>
    <value>/persist/hadoop/tmp</value>
  </property>
  <property>
    <name>fs.defaultFS</name>
    <value>hdfs://MASTER_ADDRESS:12345/</value>
  </property>
</configuration>
          `,
          'config/hdfs-site.xml': `
<configuration>
  <property>
    <name>dfs.namenode.name.dir</name>
    <value>file:///persist/hadoop/dfs/name</value>
  </property>
  <property>
    <name>dfs.datanode.data.dir</name>
    <value>file:///persist/hadoop/dfs/data</value>
  </property>
  <property>
    <name>dfs.namenode.datanode.registration.ip-hostname-check</name>
    <value>false</value>
  </property>
  <property>
    <name>dfs.client.use.datanode.hostname</name>
    <value>true</value>
  </property>
  <property>
    <name>dfs.datanode.use.datanode.hostname</name>
    <value>true</value>
  </property>
</configuration>
          `,
          'config/start-main.sh': `#!/bin/bash
cp -f /etc/hadoop/core-site.xml /opt/hadoop/etc/hadoop
cp -f /etc/hadoop/hdfs-site.xml /opt/hadoop/etc/hadoop
sed -i "s/MASTER_ADDRESS/$SIGMA_CONTAINER_IP/" /opt/hadoop/etc/hadoop/core-site.xml
/opt/hadoop/bin/hdfs namenode -format -nonInteractive
exec /opt/hadoop/bin/hdfs namenode
          `,
          'config/start-lookup.sh': `#!/bin/bash
while true; do
  echo '127.0.0.1 localhost' > /etc/hosts
  curl -SGsk -u $SIGMA_LOOKUP_TOKEN \
    --data-urlencode labelSelector=managed-by=hadoop-namenode-${name} \
    $SIGMA_LOOKUP_URL \
    | jq -r '.items[]|"\\(.status.podIP) \\(.metadata.name)"' >> /etc/hosts
  curl -SGsk -u $SIGMA_LOOKUP_TOKEN \
    --data-urlencode labelSelector=managed-by=hadoop-datanode-${name} \
    $SIGMA_LOOKUP_URL \
    | jq -r '.items[]|"\\(.status.podIP) \\(.metadata.name)"' >> /etc/hosts
  sleep 10
done
          `,
        },
      },
      spec: {
        nodeSelector: { project: 'default' },
        containers: [
          {
            name: '',
            image: '61.160.36.122:8080/hadoop-bin:2.7.2-2',
            ports: [
              { name: 'ssh', containerPort: 22 },
              { name: 'http', containerPort: 50070 },
            ],
            env: [
              { name: 'SSH_PUBLIC_KEY', value: '' },
              { name: 'SIGMA_CONTAINER_IP', valueFrom: { fieldRef: { fieldPath: 'status.podIP' } } },
              { name: 'SIGMA_CONTAINER_NAME', valueFrom: { fieldRef: { fieldPath: 'metadata.name' } } },
              { name: 'SIGMA_PROJECT_NAME', valueFrom: { fieldRef: { fieldPath: 'metadata.namespace' } } },
              { name: 'SIGMA_API_SERVER', value: '61.160.36.122' },
              { name: 'SIGMA_LOOKUP_URL',
                value: 'https://$(SIGMA_API_SERVER)/api/v1/namespaces/$(SIGMA_PROJECT_NAME)/pods' },
              { name: 'SIGMA_LOOKUP_TOKEN', value: 'test:test123' },
            ],
            resources: {
              limits: { cpu: '10', memory: '20Gi' },
              requests: { cpu: '1', memory: '2Gi' },
            },
            lifecycle: {
              postStart: {
                exec: {
                  command: [
                    'sh', '-c', [
                      'cd /etc/service/hadoop',
                      'cp start.sh run',
                      'chmod +x run',
                      'cd /etc/service/hadoop-lookup',
                      'cp start.sh run',
                      'chmod +x run',
                    ].join(' && '),
                  ],
                },
              },
            },
            volumeMounts: [
              { name: 'hostinfo', mountPath: '/home/dspeak/yyms', readOnly: true },
              { name: 'localtime', mountPath: '/etc/localtime', readOnly: true },
              { name: 'yy-repos', mountPath: '/usr/local/i386', readOnly: true },
              { name: 'yymp-agent-sock', mountPath: '/tmp/yymp.agent.sock' },
              { name: 'persist', mountPath: '/persist' },
              { name: 'site-config', mountPath: '/etc/hadoop' },
              { name: 'start-main', mountPath: '/etc/service/hadoop' },
              { name: 'start-lookup', mountPath: '/etc/service/hadoop-lookup' },
            ],
          },
        ],
        volumes: [
          { name: 'hostinfo', hostPath: { path: '/home/dspeak/yyms' } },
          { name: 'localtime', hostPath: { path: '/etc/localtime' } },
          { name: 'yy-repos', hostPath: { path: '/usr/local/i386' } },
          { name: 'yymp-agent-sock', hostPath: { path: '/tmp/yymp.agent.sock' } },
          { name: 'persist', emptyDir: { medium: '' } },
          { name: 'site-config', downwardAPI: { items: [
            { path: 'core-site.xml', fieldRef: { fieldPath: 'metadata.annotations.config/core-site.xml' } },
            { path: 'hdfs-site.xml', fieldRef: { fieldPath: 'metadata.annotations.config/hdfs-site.xml' } },
          ] } },
          { name: 'start-main', downwardAPI: { items: [
            { path: 'start.sh', fieldRef: { fieldPath: 'metadata.annotations.config/start-main.sh' } },
          ] } },
          { name: 'start-lookup', downwardAPI: { items: [
            { path: 'start.sh', fieldRef: { fieldPath: 'metadata.annotations.config/start-lookup.sh' } },
          ] } },
        ],
      },
    },
  },
});

function postForm(path, params) {
  const method = 'post';
  const form = document.createElement('form');
  form.setAttribute('method', method);
  form.setAttribute('action', path);
  Object.keys(params).forEach(key => {
    const hiddenField = document.createElement('input');
    hiddenField.setAttribute('type', 'hidden');
    hiddenField.setAttribute('name', key);
    hiddenField.setAttribute('value', params[key]);
    form.appendChild(hiddenField);
  });
  document.body.appendChild(form);
  form.submit();
}

class HDFSApp extends Component {
  constructor() {
    super();
    this.state = {
      selectedConfig: 0,
    };
  }

  handleConfigClick(id) {
    this.setState({
      selectedConfig: id,
    });
  }

  handleSubmit(form) {
    const { appName, sshPublicKey } = form;
    const selectedConfig = this.state.selectedConfig;
    const namespace = window.location.pathname.split('/', 3).pop();
    const nameNode = getNameNode(appName);
    nameNode.spec.template.spec.containers[0].env[0].value = sshPublicKey;
    nameNode.spec.replicas = configList[selectedConfig].replicas;
    postForm(`/namespaces/${namespace}/replicationcontrollers`, {
      json: JSON.stringify(nameNode),
    });
  }

  render() {
    return (
      <div>
        <h1 className="page-header">创建容器化应用 - HDFS</h1>
        <ApplicationForm
          configList={configList}
          selectedConfig={this.state.selectedConfig}
          handleConfigClick={this.handleConfigClick.bind(this)}
          handleSubmit={this.handleSubmit.bind(this)}
        />
      </div>
    );
  }
}

HDFSApp.propTypes = {
  params: PropTypes.object,
  route: PropTypes.object,
};

export default HDFSApp;

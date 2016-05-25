import React, { Component, PropTypes } from 'react';
import ApplicationForm from './ApplicationForm';

let configList = [
  {
    id: 1,
    type: 'basic',
    scale: 0,
    replicas: 1,
    cpu: '1',
    memory: '2G',
  },
  {
    id: 2,
    type: 'production',
    scale: 1,
    replicas: 3,
    cpu: '2',
    memory: '4G',
  },
  {
    id: 3,
    type: 'production',
    scale: 2,
    replicas: 6,
    cpu: '2',
    memory: '4G',
  },
  {
    id: 4,
    type: 'production',
    scale: 4,
    replicas: 12,
    cpu: '2',
    memory: '4G',
  },
];

class HDFSApp extends Component {
  constructor() {
    super();
    this.state = {
      selectedConfig: 1,
    };
  }

  handleConfigClick(id) {
    this.setState({
      selectedConfig: id,
    });
  }

  handleSubmit(form) {
    console.log(`appName: ${form.appName}`);
    console.log(`sshPublicKey: ${form.sshPublicKey}`);
    console.log(`selectedConfig: ${this.state.selectedConfig}`);
    const namespace = window.location.pathname.split('/', 3).pop();
    console.log(`currentNamespace: ${namespace}`);
    // window.location.href = `/namespaces/${namespace}`;
  }

  render() {
    return (
      <div>
        <h1 className="page-header">Create Dockerized Application - HDFS</h1>
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

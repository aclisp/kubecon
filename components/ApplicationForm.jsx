import React, { Component, PropTypes } from 'react';
import ConfigurationItem from './ConfigurationItem';

class ApplicationForm extends Component {
  handleConfigClick(id) {
    this.props.handleConfigClick(id);
  }

  handleSubmit(event) {
    event.preventDefault();
    this.props.handleSubmit({
      appName: event.target.name.value,
      sshPublicKey: event.target.key.value,
    });
  }

  render() {
    let configs = this.props.configList.map((cfg, id) =>
      <ConfigurationItem
        config={cfg}
        id={id}
        key={id}
        selected={this.props.selectedConfig === id}
        onClick={this.handleConfigClick.bind(this)}
      />
    );

    return (
      <form onSubmit={this.handleSubmit.bind(this)}>
        <div className="form-group">
          <label htmlFor="name">应用实例名称</label>
          <input type="text" className="form-control" id="name" name="name" placeholder="Application name" />
        </div>
        <div className="form-group">
          <label htmlFor="key">SSH public key</label>
          <textarea className="form-control" id="key" name="key" placeholder="SSH public key" rows="5" />
        </div>
        <div className="form-group">
          <label>选择配置</label>
          <div className="container-fluid">
            <div className="row">
              {configs}
            </div>
          </div>
        </div>
        <button type="submit" className="btn btn-default">Submit</button>
      </form>
    );
  }
}

ApplicationForm.propTypes = {
  configList: PropTypes.array,
  selectedConfig: PropTypes.number,
  handleConfigClick: PropTypes.func,
  handleSubmit: PropTypes.func,
};

export default ApplicationForm;

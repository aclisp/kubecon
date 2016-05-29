import React, { Component, PropTypes } from 'react';

class ConfigurationItem extends Component {
  onClick(id) {
    this.props.onClick(id);
  }

  render() {
    let className = 'panel-default';
    if (this.props.selected) {
      className = 'panel-primary';
    }
    return (
      <div className="col-md-3" >
        <div className={`panel ${className}`} onClick={this.onClick.bind(this, this.props.id)}>
          <div className="panel-heading">
            {this.props.config.type} ({this.props.config.scale}x)
          </div>
          <div className="panel-body">
            <ul className="list-group">
              <li className="list-group-item">{this.props.config.replicas} replicas</li>
              <li className="list-group-item">{this.props.config.cpu} cores per replica </li>
              <li className="list-group-item">{this.props.config.memory} memory per replica</li>
            </ul>
          </div>
        </div>
      </div>
    );
  }
}

ConfigurationItem.propTypes = {
  config: PropTypes.object,
  id: PropTypes.number,
  selected: PropTypes.bool,
  onClick: PropTypes.func,
};

export default ConfigurationItem;

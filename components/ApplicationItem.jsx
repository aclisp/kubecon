import React, { Component, PropTypes } from 'react';
import { Link } from 'react-router';

class ApplicationItem extends Component {
  render() {
    return (
      <div className="col-md-3">
        <div className="panel panel-default">
          <div className="panel-body">
            <Link to={`/new/${this.props.app.id}`}>
              {this.props.app.title}
            </Link>
          </div>
        </div>
      </div>
    );
  }
}

ApplicationItem.propTypes = {
  app: PropTypes.object,
};

export default ApplicationItem;

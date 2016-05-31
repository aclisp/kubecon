import React, { Component, PropTypes } from 'react';

class UnimplementedApplication extends Component {
  render() {
    const title = this.props.route.apps.find((app) => app.id === this.props.params.app_id).title;

    return (
      <div>
        <h1 className="page-header">{`创建容器化应用 - ${title}`}</h1>
        <p>Coming Soon...</p>
      </div>
    );
  }
}

UnimplementedApplication.propTypes = {
  params: PropTypes.object,
  route: PropTypes.object,
};

export default UnimplementedApplication;

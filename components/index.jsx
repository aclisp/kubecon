import React, { Component, PropTypes } from 'react';
import { render } from 'react-dom';
import { Router, Route, hashHistory } from 'react-router';
import ApplicationItem from './ApplicationItem';
import UnimplementedApplication from './UnimplementedApplication';
import HDFSApp from './HDFSApp';

let appList = [
  {
    id: 'zookeeper',
    title: 'ZooKeeper',
  },
  {
    id: 'mongodb',
    title: 'MongoDB',
  },
  {
    id: 'mysql',
    title: 'MySQL',
  },
  {
    id: 'postgres',
    title: 'PostgreSQL',
  },
  {
    id: 'hdfs',
    title: 'HDFS',
  },
  {
    id: 'yarn',
    title: 'Hadoop YARN',
  },
  {
    id: 'mapreduce',
    title: 'Hadoop MapReduce',
  },
  {
    id: 'spark',
    title: 'Spark',
  },
  {
    id: 'storm',
    title: 'Storm',
  },
  {
    id: 'tensorflow',
    title: 'TensorFlow',
  },
];

class ApplicationGallery extends Component {
  render() {
    let apps = this.props.route.apps.map((app) =>
      <ApplicationItem app={app} key={app.id} />
    );

    return (
      <div>
        <h1 className="page-header">Application Gallery</h1>
        <div className="container-fluid">
          <div className="row">
            {apps}
          </div>
        </div>
        {this.props.children}
      </div>
    );
  }
}

ApplicationGallery.propTypes = {
  children: PropTypes.node,
  route: PropTypes.object,
};

render((
  <Router history={hashHistory}>
    <Route path="/" component={ApplicationGallery} apps={appList}>
      <Route path="new/hdfs" component={HDFSApp} />
      <Route path="new/:app_id" component={UnimplementedApplication} apps={appList} />
    </Route>
  </Router>
), document.getElementById('root'));

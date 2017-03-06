import reactor from 'app/reactor';
import { TLPT_NODES_RECEIVE } from './actionTypes';
import api from 'app/services/api';
import cfg from 'app/config';
import { showError } from 'app/flux/notifications/actions';
import appGetters from 'app/flux/app/getters';
import Logger from 'app/lib/logger';

const logger = Logger.create('Modules/Nodes');

export default {
  fetchNodes() {
    let siteId = reactor.evaluate(appGetters.siteId);
    return api.get(cfg.api.getSiteNodesUrl(siteId))
      .done((data = []) => {
        let nodeArray = data.nodes.map(item => item.node);                     
        reactor.dispatch(TLPT_NODES_RECEIVE, { siteId, nodeArray });
      })
      .fail(err => {
        showError('Unable to retrieve list of nodes');
        logger.error('fetchNodes', err);
    })
  }
}
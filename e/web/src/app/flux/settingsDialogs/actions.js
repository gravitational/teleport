import reactor from 'app/reactor';
import * as AT from './actionTypes';
import * as RAT from './../restApi/constants';
import apiActions from './../restApi/actions';

export function openDeleteDialog(item){
  reactor.dispatch(AT.SET_RES_TO_DELETE, item);
}

export function closeDeleteDialog(){
  apiActions.clear(RAT.TRYING_TO_DELETE_RESOURCE);
  reactor.dispatch(AT.SET_RES_TO_DELETE, null);
}
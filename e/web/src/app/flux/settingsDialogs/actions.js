import reactor from '../../reactor';
import * as AT from './actionTypes';
import { deleteResourceStatus } from './../status/actions';

export function openDeleteDialog(item){
  reactor.dispatch(AT.SET_RES_TO_DELETE, item);
}

export function closeDeleteDialog(){
  deleteResourceStatus.clear();
  reactor.dispatch(AT.SET_RES_TO_DELETE, null);
}
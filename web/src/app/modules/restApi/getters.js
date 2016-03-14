var defaultObj = {
  isProcessing: false,
  isError: false,
  isSuccess: false,
  message: ''
}

const requestStatus = (reqType) =>  [ ['tlpt_rest_api', reqType], (attemp) => {
  return attemp ? attemp.toJS() : defaultObj;
 }
];

export default {  requestStatus  };

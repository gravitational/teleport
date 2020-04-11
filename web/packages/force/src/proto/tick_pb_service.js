// package: proto
// file: tick.proto

var tick_pb = require("./tick_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var TickService = (function () {
  function TickService() {}
  TickService.serviceName = "proto.TickService";
  return TickService;
}());

TickService.Subscribe = {
  methodName: "Subscribe",
  service: TickService,
  requestStream: false,
  responseStream: true,
  requestType: tick_pb.TickRequest,
  responseType: tick_pb.Tick
};

TickService.Now = {
  methodName: "Now",
  service: TickService,
  requestStream: false,
  responseStream: false,
  requestType: tick_pb.TickRequest,
  responseType: tick_pb.Tick
};

exports.TickService = TickService;

function TickServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

TickServiceClient.prototype.subscribe = function subscribe(requestMessage, metadata) {
  var listeners = {
    data: [],
    end: [],
    status: []
  };
  var client = grpc.invoke(TickService.Subscribe, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onMessage: function (responseMessage) {
      listeners.data.forEach(function (handler) {
        handler(responseMessage);
      });
    },
    onEnd: function (status, statusMessage, trailers) {
      listeners.status.forEach(function (handler) {
        handler({ code: status, details: statusMessage, metadata: trailers });
      });
      listeners.end.forEach(function (handler) {
        handler({ code: status, details: statusMessage, metadata: trailers });
      });
      listeners = null;
    }
  });
  return {
    on: function (type, handler) {
      listeners[type].push(handler);
      return this;
    },
    cancel: function () {
      listeners = null;
      client.close();
    }
  };
};

TickServiceClient.prototype.now = function now(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(TickService.Now, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

exports.TickServiceClient = TickServiceClient;


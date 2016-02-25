var { $, reactor, expect, Dfd, spyOn, api, React, ReactDOM, ReactTestUtil } = require('./../');
var ExpanderComponent = require('app/wizard/components/aws/awsInstaller.jsx');
var Expander = require('app/wizard/stores/provisioners/aws/awsExpander');
var $node = $('<div>').appendTo("body");

describe('components/nodes', function () {
  afterEach(function () {
    ReactDOM.unmountComponentAtNode($node[0])
  })

  describe('when "info step with the server requirements" is shown', function () {
    it('should display a dropdown with available server roles', function () {
      var expander = new Expander(helpers.onprem.getActiveSite(), undefined);
      spyOn(expander, 'ensureOperationId').andReturn($.Deferred().resolve({}));
      expander.init();

      render(expander);

      var $inputs = $(".grv-wiz-req option");
      var srvCount = Object.getOwnPropertyNames(expander.siteData.app.manifest.servers).length;
      expect($node.find('.grv-wiz-req').length).toBe(1);
      expect($node.find('.grv-wiz-aws-secret').length).toBe(1);
      expect($node.find('.grv-wiz-aws-access').length).toBe(1);
      expect($inputs.length).toBe(srvCount+1);
    });
  })
});

function render(installer){
  ReactDOM.render(<ExpanderComponent installer={installer} />, $node[0]);
}

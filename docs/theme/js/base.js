function getSearchTerm() {
  var sPageURL = window.location.search.substring(1);
  var sURLVariables = sPageURL.split('&');

  for (var i = 0; i < sURLVariables.length; i++) {
    var sParameterName = sURLVariables[i].split('=');

    if (sParameterName[0] == 'q') {
      return sParameterName[1];
    }
  }
}

function isMobile() {
  if(window.innerWidth <= 800) {
    return true;
  }
  else {
    return false;
  }
}

$(document).ready(function() {
  // side nav highlighting
  var $sideNavSecondaryMenu = $(".side-nav-secondary-buttons");
  var $sideNavSecondaryButtons = $sideNavSecondaryMenu.find('a');

  $sideNavSecondaryMenu.on("click", "a", (e) => {
    var $button = $(e.currentTarget);
    var isActive = $button.hasClass('is-active');

    setTimeout(() => {
      if(!isActive) {
        $sideNavSecondaryButtons.removeClass('is-active');
        $button.addClass("is-active");
      }
    }, 50);
  });

  // activate code formatting
  if(window.PR && window.PR.prettyPrint) {
    var $preTags = $('pre');

    $preTags.each((index, el) => {
      var $pre = $(el);
      var $code = $pre.find('code');
      var lang = $code.attr('class');
      $code.removeAttr('class');

      if(!lang || lang === 'bash') {
        lang = 'bsh';
      }

      $pre.addClass(`prettyprint lang-${lang}`);
    });

    window.PR.prettyPrint();
  }

  /* Prevent disabled links from causing a page reload */
  $("li.disabled a").click((e) => {
    e.preventDefault();
  });

    // activate nav
  if (window.grvlib) {
    var topPadding = isMobile() ? 156 : 16;

    new grvlib.TopNav();
    new grvlib.SecondaryNav();
    new grvlib.SideNav({pinned: true});
    // grvlib.buttonSmoothScroll(topPadding);
    grvlib.buttonRipple();
  }
});


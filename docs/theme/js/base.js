function getSearchTerm() {
  const sPageURL = window.location.search.substring(1);
  const sURLVariables = sPageURL.split('&');

  for (let i = 0; i < sURLVariables.length; i++) {
    let sParameterName = sURLVariables[i].split('=');

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
  // SIDE NAV HIGHLIGHTING
  const $sideNavSecondaryMenu = $(".side-nav-secondary-buttons");
  const $sideNavSecondaryButtons = $sideNavSecondaryMenu.find('a');

  $sideNavSecondaryMenu.on("click", "a", (e) => {
    const $button = $(e.currentTarget);
    const isActive = $button.hasClass('is-active');

    setTimeout(() => {
      if(!isActive) {
        $sideNavSecondaryButtons.removeClass('is-active');
        $button.addClass("is-active");
      }
    }, 50);
  });

  // ACTIVATE CODE FORMATTING
  if(window.PR && window.PR.prettyPrint) {
    let $preTags = $('pre');

    $preTags.each((index, el) => {
      const $pre = $(el);
      const $code = $pre.find('code');
      let lang = $code.attr('class');
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

    // ACTIVATE NAVIGATION
  if (window.grvlib) {
    let topPadding = isMobile() ? 156 : 16;

    new grvlib.TopNav();
    new grvlib.SecondaryNav();
    new grvlib.SideNav({pinned: true});
    // grvlib.buttonSmoothScroll(topPadding);
    grvlib.buttonRipple();
  }
});


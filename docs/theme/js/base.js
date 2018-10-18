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

$(document).ready(function() {
  // ACTIVATE NAVIGATION
  if (window.grvlib) {
    new grvlib.TopNav();
    new grvlib.SecondaryNav();
    grvlib.buttonSmoothScroll();
    grvlib.buttonRipple();
  }

  // activate code prettifier
  if(window.PR && window.PR.prettyPrint) {
    let $preTags = $('pre');

    $preTags.each((index, el) => {
      const $pre = $(el);
      const $code = $pre.find('code');
      const lang = $code.attr('class');

      $pre.addClass(`prettyprint lang-${lang}`);
    });

    window.PR.prettyPrint();
  }

  $('table').addClass('table table-striped table-hover');


  /* Prevent disabled links from causing a page reload */
  $("li.disabled a").click((e) => {
    e.preventDefault();
  });
});


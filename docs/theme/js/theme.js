
(function ($) {
  /**
   * Copyright 2012, Digital Fusion
   * Licensed under the MIT license.
   * http://teamdf.com/jquery-plugins/license/
   *
   * @author Sam Sehnert
   * @desc A small plugin that checks whether elements are within
   *       the user visible viewport of a web browser.
   *       only accounts for vertical position, not horizontal.
   */
  var $w = $(window);
  $.fn.visible = function(partial, hidden, direction, container) {
    if (this.length < 1) return;

    // Set direction default to 'both'.
    direction = direction || "both";

    var $t = this.length > 1 ? this.eq(0) : this,
      isContained = typeof container !== "undefined" && container !== null,
      $c = isContained ? $(container) : $w,
      wPosition = isContained ? $c.position() : 0,
      t = $t.get(0),
      vpWidth = $c.outerWidth(),
      vpHeight = $c.outerHeight(),
      clientSize = hidden === true ? t.offsetWidth * t.offsetHeight : true;

    if (typeof t.getBoundingClientRect === "function") {
      // Use this native browser method, if available.
      var rec = t.getBoundingClientRect(),
        tViz = isContained
          ? rec.top - wPosition.top >= 0 && rec.top < vpHeight + wPosition.top
          : rec.top >= 0 && rec.top < vpHeight,
        bViz = isContained
          ? rec.bottom - wPosition.top > 0 &&
            rec.bottom <= vpHeight + wPosition.top
          : rec.bottom > 0 && rec.bottom <= vpHeight,
        lViz = isContained
          ? rec.left - wPosition.left >= 0 &&
            rec.left < vpWidth + wPosition.left
          : rec.left >= 0 && rec.left < vpWidth,
        rViz = isContained
          ? rec.right - wPosition.left > 0 &&
            rec.right < vpWidth + wPosition.left
          : rec.right > 0 && rec.right <= vpWidth,
        vVisible = partial ? tViz || bViz : tViz && bViz,
        hVisible = partial ? lViz || rViz : lViz && rViz,
        vVisible = rec.top < 0 && rec.bottom > vpHeight ? true : vVisible,
        hVisible = rec.left < 0 && rec.right > vpWidth ? true : hVisible;

      if (direction === "both") return clientSize && vVisible && hVisible;
      else if (direction === "vertical") return clientSize && vVisible;
      else if (direction === "horizontal") return clientSize && hVisible;
    } else {
      var viewTop = isContained ? 0 : wPosition,
        viewBottom = viewTop + vpHeight,
        viewLeft = $c.scrollLeft(),
        viewRight = viewLeft + vpWidth,
        position = $t.position(),
        _top = position.top,
        _bottom = _top + $t.height(),
        _left = position.left,
        _right = _left + $t.width(),
        compareTop = partial === true ? _bottom : _top,
        compareBottom = partial === true ? _top : _bottom,
        compareLeft = partial === true ? _right : _left,
        compareRight = partial === true ? _left : _right;

      if (direction === "both")
        return (
          !!clientSize &&
          (compareBottom <= viewBottom && compareTop >= viewTop) &&
          (compareRight <= viewRight && compareLeft >= viewLeft)
        );
      else if (direction === "vertical")
        return (
          !!clientSize && (compareBottom <= viewBottom && compareTop >= viewTop)
        );
      else if (direction === "horizontal")
        return (
          !!clientSize && (compareRight <= viewRight && compareLeft >= viewLeft)
        );
    }
  };
})(jQuery);

// checks if element is fully visible
function checkVisible(elm, container) {
  return $(elm).visible( true, false, 'vertical', container );
}

// debounce
function debounce(func, wait, immediate) {
	var timeout;
	return function() {
		var context = this, args = arguments;
		var later = function() {
			timeout = null;
			if (!immediate) func.apply(context, args);
		};
		var callNow = immediate && !timeout;
		clearTimeout(timeout);
		timeout = setTimeout(later, wait);
		if (callNow) func.apply(context, args);
	};
};


// finds currently visible header and highlights corresponding menu item
function handleNavScroll() {
  var $container = $(window);
  var $menus = $(".side-nav .side-nav-secondary-buttons");
  var $targets = $("#docs-content").find('[id]');
  var activeClass = 'is-active';
  var linkMap = {};

  $menus.find("a").each(function (i, value) {
    var $value = $(value);
    var href = $value.attr('href').replace('#', '');
    linkMap[href] = $value;
  })

  function hasMenuItem(id) {
    return !!linkMap[id];
  }

  function selectMenuItem(id) {
    if (!hasMenuItem(id)) {
      return;
    }

    var $link = $(linkMap[id]);
    $menus.find('.'+activeClass).removeClass(activeClass);
    $link.addClass(activeClass)
  }

  function findAndActivateClosest() {
    for (var i = $targets.length-1; i > 0; i--){
      var a = window.scrollY;
      var b = $targets.eq(i).position().top;
      if (b > a) {
        continue;
      }

      var id = $targets.eq(i).attr('id');
      if (hasMenuItem(id)) {
        selectMenuItem(id);
        return;
      }
    }
  }

  function updateMenu() {
    for (var i = 0; i < $targets.length; i++) {
      var id = $targets.eq(i).attr('id');
      if (checkVisible($targets.eq(i)) && hasMenuItem(id) ) {
        selectMenuItem(id);
        return;
      }
    }

    findAndActivateClosest();
  }

  var hash = window.document.location.hash;
  if (!hash) {
    updateMenu();
  } else {
    selectMenuItem(hash.replace('#', ''));
  }

  $container.scroll(debounce(updateMenu, 50));
}


// creates version selector
function handleVerSelector() {
  if (!window.grvConfig || !window.grvConfig.docVersions) {
    return;
  }

  var docVersions = window.grvConfig.docVersions || [];
  var docCurrentVer = window.grvConfig.docCurrentVer;
  var viewingLatest = docVersions.length === 0 || docCurrentVer === docVersions[docVersions.length - 1];
  var $content = $("#docs-content");
  var $versionContainer = $content.find('h1').first();

  function getVerUrl(ver, isLatest) {
    // looks for version number and replaces it with new value
    // ex: http://host/docs/ver/1.2/review -> http://host/docs/ver/4.0
    var reg = new RegExp("\/ver\/([0-9|\.]+(?=\/.))");
    var url = window.location.href.replace(reg, '');
    var newPrefix = isLatest ? "" : "/ver/" + ver +"/";
    return url.replace(mkdocs_page_url, newPrefix);
  }

  // Generate options
  var $options = [];
  for (var i = 0; i < docVersions.length; i++) {
    var ver = docVersions[i];
    var $li = null;
    var isCurrent = docCurrentVer === ver;

    if (isCurrent) {
      curValue = ver;
      $options.push('<option selected value="' + ver + '" >Version ' + ver + "</option>"  );
      continue;
    }

    var isLatest = docVersions.indexOf(ver) === (docVersions.length - 1);
    var baseUrl = getVerUrl(ver, isLatest);
    $options.push(' <option value="' + baseUrl + '" >Version ' + ver + "</option>");
  }

  // show version list
  var $versionList = $(`
    <form class="version-selector">
      <select class="form-select" name="menu" onChange="window.document.location.href=this.options[this.selectedIndex].value;" value="' + curValue + '">
        ${$options.reverse().join('')}
      </select>
    </form>
  `);
  $versionContainer.prepend($versionList);

  // show warning for old versions
  if (!viewingLatest) {
    var latestVerUrl = getVerUrl(docVersions[docVersions.length - 1], true);
    $content.prepend(`
      <div class="notice is-error">
        <header>Version Warning</header>
        <div class="notice-content">
          <p>
          This chapter covers Teleport ${docCurrentVer}. We highly recommend evaluating
          the <a href="${latestVerUrl}">latest</a> version instead.
          </p>
        </div>
      </div>
    `);
  }
}

// append sub-anchors to the H2 and H3 elements for one-click linking:
function handleHeaderLinks(){
  $('#docs-content').find("h2, h3").each(function () {
    var $e = $(this);
    $e.append("<a href='#" + $e.attr("id") + "'></a>");
  });
}

function init(fn, description) {
  try {
    fn()
  } catch (err) {
    console.error('failed to init ' + description, err);
  }
}

$(document).ready(function () {
  init(handleHeaderLinks, "handleHeaderLinks");
  init(handleVerSelector, "handleVerSelector");
  init(handleNavScroll, "handleNavScroll");
});
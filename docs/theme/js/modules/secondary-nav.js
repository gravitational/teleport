// dependancies
import $ from 'jquery';
import _ from 'lodash';

class SecondaryNav {
  constructor(id) {
    const elementId = id || '#secondary-nav';

    // register elements & vars
    this.$window = $(window);
    this.$secondaryNav = $(elementId);
    this.$trigger = this.$secondaryNav.find('#secondary-nav-trigger');
    this.$close = this.$secondaryNav.find('#secondary-nav-close');
    this.$menu = this.$secondaryNav.find('#secondary-nav-menu');
    this.$buttons = this.$secondaryNav.find('.secondary-nav-button');
    this.currentPath = window.location.pathname;
    this.productName =  this.$secondaryNav.data('name');

    // activate navigation
    this.activateMenuHighlights();
    this.activateMobileMenu();
    this.pinSecondaryNav();
  }

  activateMenuHighlights() {
    const that = this;

    this.$buttons.each(function(index, el) {
      const $button = $(el);
      const href = $button.attr('href');
      const path = href.replace(/\.\.\//g, '');
      const paths = _.split(path, '/');

      if(that.currentPath === `/${path}`) {
        $button.addClass('is-active');
      }
      else if(that.currentPath.includes(`/${path}`) && paths.length >= 3) {
        $button.addClass('is-active');
      }
    });
  }

  activateMobileMenu() {
    this.$trigger.on('click', (e) => {
      e.preventDefault();

      this.$trigger.addClass('is-hidden')
      this.$close.addClass('is-visible')
      this.$menu.addClass('is-visible')
    });

    this.$close.on('click', (e) => {
      e.preventDefault();

      this.$trigger.removeClass('is-hidden')
      this.$close.removeClass('is-visible')
      this.$menu.removeClass('is-visible')
    });
  }

  pinSecondaryNav() {
    if (this.$window[0].pageYOffset > 2) {
      this.$secondaryNav.addClass("is-fixed");
    }

    this.$window.on("scroll", () => {
      if (this.$window[0].pageYOffset > 200) {
        this.$secondaryNav.addClass("is-fixed");
      }
      else {
        this.$secondaryNav.removeClass("is-fixed");
      }
    });
  }
}

export default SecondaryNav;
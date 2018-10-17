// dependancies
import $ from 'jquery';

class TopNav {
  constructor(id) {
    const elementId = id || '#top-nav';

    // register elements & vars
    this.$window = $(window);
    this.$nav = $(elementId);
    this.$trigger = this.$nav.find('#top-nav-trigger');
    this.$close = this.$nav.find('#top-nav-close');
    this.$menu = this.$nav.find('#top-nav-menu');
    this.$cta = this.$nav.find('#top-nav-cta');
    this.$dropDownButtons = this.$nav.find('.top-nav-button.has-dropdown');
    this.$overlays = this.$nav.find('.top-nav-dropdown-overlay');
    this.currentPath = window.location.pathname;

    // activate event listeners
    this.activateDropdownMenus();
    this.activateMobileMenu();
    this.pinTopNav();
    this.updateCta();
  }

  activateDropdownMenus() {
    // listen for dropdown button click
    this.$dropDownButtons.on('click', (e) => {
      e.stopImmediatePropagation();
      const $button = $(e.currentTarget);
      const $dropdown = $button.find('.top-nav-dropdown');
      const $overlay = $button.find('.top-nav-dropdown-overlay');

      $button.toggleClass('is-active');
      $overlay.toggleClass('is-hidden');
      $dropdown.toggleClass('is-hidden');
    });

    // close menus when overlay is clicked
    this.$overlays.on('click', (e) => {
      e.stopImmediatePropagation();
      const $overlay = $(e.currentTarget);
      const $dropdown = $overlay.siblings('.top-nav-dropdown');
      const $button = $overlay.parent();

      $button.toggleClass('is-active');
      $dropdown.toggleClass('is-hidden');
      $overlay.toggleClass('is-hidden');
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

  pinTopNav() {
    if (this.$window[0].pageYOffset > 2) {
      this.$nav.addClass("is-fixed");
    }

    this.$window.on("scroll", () => {
      if (this.$window[0].pageYOffset > 200) {
        this.$nav.addClass("is-fixed");
      }
      else {
        this.$nav.removeClass("is-fixed");
      }
    });
  }

  updateCta() {
    // change cta to teleport demo on teleport pages
    if (this.$cta.length && this.currentPath.includes('/teleport/')) {
      this.$cta.attr('href', '/teleport/demo/');
      this.$cta.text('Demo Teleport');
    }

    // change cta to telekube demo on telekube pages
    if (this.$cta.length && this.currentPath.includes('/gravity/')) {
      this.$cta.attr('href', '/gravity/demo/');
      this.$cta.text('Demo Gravity');
    }
  }
}

export default TopNav;
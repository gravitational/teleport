# [FAMFAMFAM flag icons](http://tkrotoff.github.com/famfamfam_flags/)

[FAMFAMFAM flag icons](http://famfamfam.com/lab/icons/flags/) by Mark James.

To get started, checkout http://tkrotoff.github.com/famfamfam_flags/

If you are using Ruby on Rails, check https://github.com/tkrotoff/famfamfam_flags_rails

## Sprite generation using glue

    rm bl.png bv.png famfamfam-flags.png gf.png hm.png mf.png re.png sj.png um.png
    glue . . --sprite-namespace= --namespace=famfamfam-flag --each-template="%(class_name)s { background-position: %(x)s %(y)s; width: %(width)s; height: %(height)s; }\n"

## License

As stated on http://famfamfam.com/lab/icons/flags/: "These flag icons are available for free use for any purpose with no requirement for attribution."

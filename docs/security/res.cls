% RESUME DOCUMENT STYLE -- Released 23 Nov 1989
%    for LaTeX version 2.09
% Copyright (C) 1988,1989 by Michael DeCorte

\typeout{Document Style `res' <26 Sep 89>.}

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% res.sty
%
% \documentstyle{res}
%
% Copyright (c) 1988 by Michael DeCorte
% Permission to copy all or part of this work is granted, provided
% that the copies are not made or distributed for resale, and that
% the copyright notice and this notice are retained.
%
% THIS WORK IS PROVIDED ON AN "AS IS" BASIS.  THE AUTHOR PROVIDES NO
% WARRANTY WHATSOEVER, EITHER EXPRESS OR IMPLIED, REGARDING THE WORK,
% INCLUDING WARRANTIES WITH RESPECT TO ITS MERCHANTABILITY OR FITNESS
% FOR ANY PARTICULAR PURPOSE.
%
% If you make any improvements, I'd like to hear about them.
%
% Michael DeCorte
% P.O. Box 652
% Potsdam NY 13676
% mrd@sun.soe.clarkson.edu
% mrd@clutx.bitnet
%
% Changes for LaTeX2e -- Venkat Krishnamurthy (Aug 7, 2001)
%
%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% You can have multiple style options the legal options ones are:
%
%   centered	the name and address are centered at the top of the
%		page (default)
%
%   line	the name is the left with a horizontal line then 
%		the address to the right
%
%   overlapped	the section titles overlap the body text (default)
%
%   margin	the section titles are to the left of the body text
%		
%   11pt	use 11 point fonts instead of 10 point fonts
%
%   12pt	use 12 point fonts instead of 10 point fonts
%
%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
%   Commands
%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
%   \Resume	prints the word resume but typeset nicely
%
%   \newsectionwidth{dimen}
%		defines the amount of space the labels extend
%		into the left margin.
%		DO NOT TRY to change any of the dimensions
%		yourself.  You will probably confuse the style file.
%
%   \name{text} defines your name
%
%   \address{text}
%		defines your address
%		this can be called twice if you have two addresses
%		use \\'s to indicate where either line breaks or
%		comas should go
%
%   \opening	this prints your name and address at that spot
%		this is not normally needed, as \begin{resume}
%		does this but is provided just in case you need
%		to do something odd
%
%   \begin{resume} ... \end{resume}
%		all of the resume should go inside of this
%		environment
%
%   \section{text}
%		This prints 'text' in the left hand margin.
%		Its exact placement depends on what the style 
%		options has been set to. (overlapped or margin)
%		You should use \\ to start a new line.	If the
%		style option is margin, the \\ is converted
%		to a space.
%		To use this in any of the list environments, put
%		the \section after the \item[] but before the 
%		text.
%		Eg.
%		\begin{itemize}
%		\item\section{text}
%		text
%		\end{itemize}
%
%   \begin{ncolumn}{n} ... \end{ncolumn}
%		creates a tabular environment with n equally
%		spaced columns.  Separate columns by & and
%		end them with \\
%
%   \begin{position} ... \end{position}
%		this is used to print a job description.  There should
%		be only one job description in it.  Information
%		related to the job (such as title, dates...) will
%		be printed.
%
%   \begin{format} ... \end{format}
%		used to change the default format for the position
%		environment.  Within it the recognized commands are:
%		\title{option}
%		\employer{option}
%		\location{option}
%		\dates{option}
%		\body
%		\\
%		where option is one of l,r,c standing for left, right, center.
%		The format will eventually be used to make several
%		tabular environments and you are defining the number of columns
%		and the placement of text within the columns of the tabulars.
%		Each row is terminated by a \\.  Any number of options can 
%		be on a line, they will each be set in their own columns.
%		Any of the options except \body may be left out.
%
%		Eg.
%		\begin{format}
%		\title{l}\employer{r}\\
%		\dates{r}\\
%		\body\\
%		\location{l}\\
%		\end{format}
%
%		In this example the title and employer information
%		are set in 2 columns left justified and right justified
%		respectively.  Then the date is set right justified.
%		Then the body is set.  Then the location is set left
%		justified.
%
% \employer{text}
% \title{text}
% \dates{text}
% \location{text}
%		declare text for the next invocation of the position
%		environment
%
%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% Glue
%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
%
% sectionskip	the amount of horizontal before a section
%
%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% Dimensions
%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
%
% sectionwidth	the amount that the section titles go in the
%		left margin
%
% resumewidth	the width of the total resume from the left
%		margin to the right.  Don't use textwidth
%
%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% Definitions
%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
%
% sectionfont	the font used to print section titles
%		use \renewcommand to change it
%
% namefont	the font used to print your name
%		use \renewcommand to change it
%
%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% THINGS TO DO
%
% add lm,rm options to format style to allow things to be placed
% in the left or right margin respectivly
%
% add capability so that \body doesn't have to be proceeded (followed)
% by a \\ allowing part of the description (eg. location) to be the
% first (last) thing of the body
%
% clean up the list building procedures
%
% write docs to tell people how to use this

\NeedsTeXFormat{LaTeX2e}[1995/12/01]
\ProvidesClass{res}[2000/05/19 v1.4b Resume class]

%\DeclareOption{11pt}{\renewcommand\@ptsize{1}}
%\DeclareOption{12pt}{\renewcommand\@ptsize{2}}

\PassOptionsToClass{11pt,12pt}{article}
\LoadClassWithOptions{article}

\newif\if@line
\newif\if@margin

\DeclareOption{line}{\@linetrue}
\DeclareOption{centered}{\@linefalse}

\DeclareOption{margin}{\@margintrue}
\DeclareOption{overlapped}{\@marginfalse}

\ExecuteOptions{overlapped,centered}
\ProcessOptions\relax

\nofiles	     % resume's don't need .aux files


\newtoks\tabular@text		    % holds the current list being processed
\newtoks\tabular@head		    % holds the head tabular list
\newtoks\tabular@tail		    % holds the tail tabular list
\newtoks\@ta			    % used by \@append
\newtoks\undefined@token\undefined@token={}

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% prints a centered name with the address centered
% or the two address on opposite sides of the page
%
\def\@printcentername{\begingroup
  % print the name centered
  \leavevmode\hbox to \textwidth{\hfil\@tablebox{\namefont\@name}\hfil}\par
  \@ifundefined{@addressone}{%
    % do nothing
  }{%
    \@ifundefined{@addresstwo}{
      % only one address
      \leavevmode\hbox to \textwidth{\hfil\@tablebox{\@addressone}\hfil}\par
    }{
      % two addresses
      \leavevmode\hbox to \textwidth{\@tablebox{\@addressone}\hfil
				     \@tablebox{\@addresstwo}}\par
    }%
  }%
\endgroup}

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% this is used to print the name and address at the top of
% the page with a hline in between
%
\def\namefont{\large\bf}
\def\@linename{\begingroup
  \def\\{, }
  {\namefont\@name}
  \vskip 2pt
  \fullline
  \vskip 2pt
  % where do you live?
  \@ifundefined{@addressone}{%
    % do nothing
  }{%
    \leavevmode\hbox to \textwidth
      {\hfill\vbox{\hbox{\@addressone}
		   \hbox{\@addresstwo}
		  }%
      }\par
  }
\endgroup}

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% HEADINGS:
% There are two types of headings:
% 1) one with the name centered and the address centered or
%    in the left and right side if there are two address
% 2) one where the name is in the upper left corner 
%    the a line accross the paper
%    then the address all on one line in the right corner
%    the second address will be directly below the first if defined
%
\let\print@name\relax
\def\ds@centered{\ifx\print@name\relax\let\print@name\@printcentername\fi}
\def\ds@line{\ifx\print@name\relax\let\print@name\@linename\fi}


%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% Use this to set the sectionwidth.
% It adjust the width of the text as well as the hoffset
% You probably shouldn't touch any of the size paramaters
% unless you really understand all of this but it is not
% hard.  Either way, it can only be executed once
%
\def\sectionfont{\bf}
\newdimen\sectionwidth
\newskip\sectionskip
\newdimen\resumewidth

\resumewidth=6.5in
\sectionskip=3.5ex plus 1ex minus -.2ex % values stolen from LaTeX

\def\newsectionwidth#1{%
		     \sectionwidth=#1
		     \textwidth=\resumewidth
		     \advance\textwidth-\sectionwidth
		     \hsize=\textwidth
		     \hoffset=\sectionwidth
}


%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% This is for sectiontitles that are entirely in the left margin.
% multiline sectiontitles are permited and will be broken by
% \TeX{} to fit into a box \verb|\sectionwidth| wide.  It is advised
% that \verb|\\| be used to break the lines by hand as \TeX{} will
% probably not do what you want.
%
% When using this with {\tt tabbing} and {\tt list} (or anything
% that is made out of {\tt list}) put the \section's inside of
% the \verb|\begin{}|  and the \verb|\item|Eg.
% \begin{verbatim}
% \begin{trivlist}
% \item[]
% \section{foo}
% text
% \end{trivlist}
% \end{verbatim}
%
\def\boxed@sectiontitle#1{%
  % this macro may be called in a tabular.  Special code must be written
  % to accomodate this.  In LaTeX, a tabular is made out of hboxes.
  % TeX never goes into horizontal mode because of this; it only
  % gets into vertical mode and restricted horizontal mode.  Certain 
  % indenting problems must be handled because of this.  They
  % are delt with at the end of this routine.
  % It is also necessary to close the hbox that was created before
  % the \section and create a new one when this macro has ended.
  % This macro therefore simulates a \kill, so that any text before
  % the \section not be printed.  The proper use is therefore
  % ...
  % text\\
  % \section{foo}
  % more text\\
  % ...
  \ifx\\\@tabcr    % is this in a tabular? (this *should* work but is a cludge)
    \@stopfield    % the is the first part of a \kill
   \else
     \@@par	     % This will end the previous paragraph if needed and
		   % go into vertical mode.  If this was already in
		   % vertical mode then the \par does nothing.

  \fi
  \begingroup
  \everypar={}%
  \def\par{\@@par}%
  \let\\=\@normalcr
  \addpenalty{\@secpenalty} % this would be a good place for a page break
			    % \@secpenalty is what LaTeX uses before its
			    % section's.  It happens to be -300
  \addvspace{\sectionskip}  % put in a bit of glue
  % The following hbox will be contributed to the page list without going
  % into horizontal mode.  Therefore, any \parindent's, \parshape's, \leftskip
  % will be ignored but \hoffset's are not.  The result is that the box will
  % only by \hoffset. This is what I want
  \hbox to 0pt{%
    \hss	 % this is an llap.  In other words, this glue
		 % will shrink by the width of the stuff in the vbox
		 % (\sectionwidth) into the left margin and then
		 % insert the contents of the vbox.
    \vtop to 0pt{% make a 0pt height paragraph, with the baseline at the
		 % lined up with the baseline of the first box in the list
      \leftskip=0pt
      \hsize=\sectionwidth
      \textwidth=\sectionwidth
      \raggedright     % you don't want this filled out to the right margin
      \sectionfont
      #1\vss	       % Go into horizontal mode; do the paragraph;
		       % go into vertical mode; add some negative glue 
		       % to give a box of 0pt height and depth
    }%
  }%
  \addpenalty{-\@secpenalty} % this would be a bad place for a page break
  \vskip-\baselineskip % when the next box is processed, baselineskip glue
		       % glue will be added (the box has no depth because of
		       % the \vss; therefore, we don't have to worry about
		       % \lineskiplimit).  This -\baselineskip glue
		       % is to undo this.  \nointerlineskip doesn't
		       % work because the baseline of this line would be lined
		       % up with the top of the top of the next box.  We
		       % want the baselines lined up.
		       %
		       % It may have been possible to do this by forcing the
		       % baseline of this box to be the top of the box but
		       % then the interline skip between this box and the
		       % previous box would be off as the baselines of the
		       % this box (the one that being made above) and the
		       % previous line would be separated by \baselineskip
		       % (probably, it may be separted by the depth of the
		       % previous box + \lineskip) but as the baseline of
		       % this box has been moved to the top, the box's would
		       % separted by to much glue.  The exact amount being
		       % the height of this box.
  \endgroup
  \ifx\\\@tabcr   % is this in a tabular? (this *should* work but is a cludge)
    % this is the second part of the \kill; it starts the next tabbing line
    % Because this routine will never get into paragraph mode when used in
    % tabbing the \parskip that is discussed below will never be inserted.
    % Therefore it should not be negated as done below.
    \@startline
    \ignorespaces
  \else
    \vskip-\parskip    % The next thing to be contributed will be a paragraph.
		       % Right before being contributed though a \vskip\parskip
		       % will be inserted.  This is to negate it.
		       %
		       % I do consider this to be a bit of a cludge but
		       % I can not find a way to write \unskipfutureskip
		       % or a way to make TeX think that nothing has
		       % been contributed to the page list.
  \fi
}


%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% This is for sectiontitles that are entirely above the section text
%
\def\unboxed@sectiontitle#1{%
  \ifx\\\@tabcr % see boxed@sectiontitle for explation
    \@stopfield
   \else
     \@@par
  \fi
  \begingroup
  \everypar={}%
  \def\par{\@@par}%
  \def\\{ }
  \addpenalty{\@secpenalty}
  \addvspace{\sectionskip}
  \hbox to 0pt{\hss\hbox to \sectionwidth{\sectionfont#1\hss}}
  \addpenalty{-\@secpenalty} % this would be a bad place for a page break
  \endgroup
  \ifx\\\@tabcr   % see boxed@sectiontitle for explation
    \@startline
  \else
    \vskip-\parskip
  \fi
  \ignorespaces
}

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% There are two types of section headings:
% 1) the section heading is all on one line and directly
%    below it, is the body of the text
% 2) the section heading is entirely in the left margin
%    (possibly taking multiple lines) with the body of
%    the text next to it
%
\let\@@section\relax
\def\ds@overlapped{\ifx\@@section\relax\newsectionwidth{0.5in}\let
  \@@section\unboxed@sectiontitle\fi}
\def\ds@margin{\ifx\@@section\relax\newsectionwidth{1.3in}\let
  \@@section\boxed@sectiontitle\fi}

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% DEFAULTS: (some of them)
%
% centered name
% overlapped section titles
%
% format is:
%    title	 employer
%    location	 dates
%    body
% with everything in the left of its column

%\input article.sty

\if@line\ds@line\else\ds@centered\fi
\if@margin\ds@margin\else\ds@overlapped\fi


%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% typeset resume all nice and pretty
%
\def\Resume{{R\'{e}sum\'{e}}}

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% makes a line of width \textwidth starting at -\hoffset
%
\def\fullline{		% hrules only listen to \hoffset
  \nointerlineskip	% so I have this code	  
  \moveleft\hoffset\vbox{\hrule width\textwidth} 
  \nointerlineskip
}

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% create a multiline box.
%
\def\@tablebox#1{\begin{tabular}[t]{@{}l@{\extracolsep{\fill}}}#1\end{tabular}}

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% use this to define your name
%
\def\name#1{\def\@name{#1}}

\def\@name{}

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% use this to define your address, this may be called more than once.
%
\let\@addressone\relax
\let\@addresstwo\relax

\def\address#1{
  \@ifundefined{@addressone}{\def\@addressone{#1}}{\def\@addresstwo{#1}}}

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% if you want to print your name and address is a slightly
% different format than sugessted, then this can be used
% to place it exactly where you want
%
\def\opening{\def\@opening{}
  \begingroup
  \leftskip=-\hoffset	     % I use leftskip to move things to the left as 
  \advance\textwidth\hoffset % changing hoffset doesn't work. But this
  \hsize=\textwidth	     % doesn't really work as hboxes are rules
			     % are unaffeted
  \let\par=\@@par
  \parindent=0pt
  \parskip=0pt
  \print@name
  \endgroup
}

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% all of the resume goes in the resume environment
%
\newenvironment{resume}{\begingroup
		       \@ifundefined{@opening}{\opening}{}
}{\endgroup}

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% gives you a tabular environment with n equally spaced columns
% \begin{ncolumn}{#} ... \end{ncolumn}
%
% The p option of LaTeX is broken in all but the newest verion
% of latex.tex, this is how to fix it
%
\def\@endpbox{\par\egroup\hfil}
\let\@@endpbox=\@endpbox

\newdimen\@columnwidth	  % the width of each column equal to
\def\ncolumn#1{%
  % \@columnwidth = \textwidth / #1
  \@columnwidth=\textwidth \divide\@columnwidth by #1
  \begin{tabular*}{\textwidth}[t]%
	{*{#1}{@{}p{\@columnwidth}@{\extracolsep{\fill}}}}
}

\def\endncolumn{\end{tabular*}}


%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
%   \employer{text} defines employer to be text
%   \location{text} defines location to be text
%   \dates{text}    defines dates    to be text
%   \title{text}    defines title    to be text
%   \body
%

\def\employer#1{\def\@employer{\print@employer{#1}}}
\def\location#1{\def\@location{\print@location{#1}}}
\def\dates#1{\def\@dates{\print@dates{#1}}}
\def\title#1{\def\@title{\print@title{#1}}}

\let\l@justify\raggedright
\let\r@justify\raggedleft
\let\c@justify\centering


%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% \@format{name}{justify}
% will define \print@#1 to print it's one argument 
% justified according to #2 which can be
%	  l = left
%	  r = right
%	  c = center
%
% eg.
%    \@format{employer}{c}
%    is the same as \def\print@employer#1{{\centering #1\par}}
%
\def\@format#1#2{%
  \expandafter\gdef\csname print@#1\endcsname##1{%
    {\csname#2@justify\endcsname##1\par}}
}

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% this is used to define how the position environment should
% formated.
%
% \begin{format} positioning text \end{format}
% where positioning text may be
%  \employer{pos}
%  \location{pos}
%  \dates{pos}
%  \title{pos}
%  \body	    (for the body of the position environment)
%   where pos is 
%     l for left
%     r for right
%     c for center
% use \\ to break the line
% you don't have to use all of the options.
% on any one line, you should indicate what you want on that line
% and where it should go within its column.
% eg.
% the following prints the employer in the left with the location
% centered within that its column.  It then prints the date on the
% right.  Then it prints the body of the position environment. Then
% it prints the title centered within its column; as there is only
% one column here it is really just centered.
%
% \begin{format}
% \employer{l}\location{c}\\
% \dates{r}\\
% \body\\
% \title{c}\\
% \end{format}

\newcounter{numberofcolumns}
\newenvironment{format}{%
  \def\end@line@head{\append@tabular@head{tabular@text}\tabular@text={}%
    \c@numberofcolumns 0}
  \def\end@line@tail{\append@tabular@tail{tabular@text}\tabular@text={}%
    \c@numberofcolumns 0}
  \tabular@text={}
  \tabular@head={}
  \tabular@tail={}
  \c@numberofcolumns 0
  \let\\=\end@line@head
  \def\employer##1{\advance\c@numberofcolumns 1
		   \@format{employer}{##1}
		   \append@tabular@text{employer}}
  \def\location##1{\advance\c@numberofcolumns 1
		   \@format{location}{##1}
		   \append@tabular@text{location}}
  \def\dates##1{\advance\c@numberofcolumns 1
		\@format{dates}{##1}
		\append@tabular@text{dates}}
  \def\title##1{\advance\c@numberofcolumns 1
		 \@format{title}{##1}
		\append@tabular@text{title}}
  \def\body{\iftoks\tabular@head\undefined@token\then
	    \else
	      \@append{\noexpand\\}\to\tabular@head
	    \skotfi
	    \let\\=\end@line@tail}}{}

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
%taken from page 378 of TeXbook but freely hacked
%
% appends the expansion of #1 to the token list #2

\def\@append#1\to#2{%
  \@ta=\expandafter{#1}%
  \xdef\@append@temp{\the#2\the\@ta}
  \global#2=\expandafter{\@append@temp}%
}


%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% CHAA006%vaxb.rhbnc.ac.uk@NSS.Cs.Ucl.AC.UK
% texhax.88.078
% is used to see if two token lists are equal
% there must be a better way
%

\let \then = \empty
\def \iftoks #1#2\then #3\else #4\skotfi{
		\edef \1{\the #1}
		\edef \2{\the #2}
		\ifx \1\2\then #3\else #4\fi}


%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% \append@tabular@text{command}
%
% appends command to the end of \tabular@text.
% NOTE: command MUST be a command but without the \
%	Eg. \append@tabular@text{relax}
%
% used to define \tabular@text for the tabular environment
% used by append@tabular@head and append@tabular@tail
%

\def\append@tabular@text#1{%
\iftoks\tabular@text\undefined@token\then
  \global\tabular@text=\expandafter{\csname @#1\endcsname}
\else
  \@append{&}\to\tabular@text
  \@append{\csname @#1\endcsname}\to\tabular@text
\skotfi
}

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% append@tabular@head
%
% appends command to the end of \tabular@text@head
% NOTE: command MUST be a command but without the \
%	Eg. \append@tabular@head{relax}
%
% used to define \tabular@head for the tabular environment
% used by the position environment
%
\def\append@tabular@head#1{%
  \ifnum\the\c@numberofcolumns=0\relax
  \else
    \iftoks\tabular@head\undefined@token\then
      \relax
    \else
      \@append{\noexpand\\}\to\tabular@head
      \@append{\noexpand\penalty-\@secpenalty}\to\tabular@head
    \skotfi
    \@append{\noexpand\begin{ncolumn}}\to\tabular@head
    \@append{\expandafter{\the\c@numberofcolumns}}\to\tabular@head
    \@append{\the\csname#1\endcsname}\to\tabular@head
    \@append{\noexpand\end{ncolumn}}\to\tabular@head
  \fi
}

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% append@tabular@tail
%
% appends command to the end of \tabular@text@htail
% NOTE: command MUST be a command but without the \
%	Eg. \append@tabular@tail{relax}
%
% used to define \tabular@tail for the tabular environment
% used by the position environment
%
\def\append@tabular@tail#1{%
  \ifnum\the\c@numberofcolumns=0\relax
  \else
    \iftoks\tabular@tail\undefined@token\then
    \else
      \@append{\noexpand\\}\to\tabular@tail
      \@append{\noexpand\penalty-\@secpenalty}\to\tabular@tail
    \skotfi
    \@append{\noexpand\begin{ncolumn}}\to\tabular@tail
    \@append{\expandafter{\the\c@numberofcolumns}}\to\tabular@tail
    \@append{\the\csname#1\endcsname}\to\tabular@tail
    \@append{\noexpand\end{ncolumn}}\to\tabular@tail
  \fi
}

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% put the actual job descriptions here
% \begin{postion} ... \end{position}
% in the ... describe the position.
% don't put the \dates \location etc in here. define them before hand
\newenvironment{position}%
  {%
   \begingroup
     \par
       \the\tabular@head
%     \addpenalty{-\@secpenalty}% bad place for a page break
     \penalty -\@secpenalty % bad place for a page break
     \penalty 10000
     \ignorespaces
  }{%
       \the\tabular@tail
%     \addpenalty{\@secpenalty}% good place for a page break
     \penalty \@secpenalty % good place for a page break
    \endgroup
}

%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%
% DEFAULTS: (the rest of them)
%
% centered name
% overlapped section titles
%
% format is:
%    title	 employer
%    location	 dates
%    body
% with everything in the left of its column

\@secpenalty = -500
\topmargin 0pt
\headheight 0pt
\headsep 0pt
\textheight 9in
\parindent 0pt
\topmargin 0in
\oddsidemargin 0pt
\evensidemargin 0pt
\marginparwidth 0pt
\parindent 0pt
\parskip \baselineskip
\setcounter{secnumdepth}{0}
\def\@listI{\leftmargin\leftmargini
\topsep 0pt 
\parskip 0pt
\partopsep 2pt plus 2pt
\parsep 2pt plus 2pt
\itemsep \parsep}

\pagestyle{empty}  % don't want page numbers

\begin{format}
\title{l}\employer{r}\\
\location{l}\dates{r}\\
\body\\
\end{format}

\let\section\@@section


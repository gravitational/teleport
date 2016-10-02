#!/bin/bash
##
## Author: guanana
## Version: 1.0
##

#Usage information
usage(){
cat <<EOF 2>&1 

Usage: $0 -f tarfile -r build-release -v build-version [-c changelog] [-s spec_dir] [-n program_name]

It generates the spec file

	-n, --program-name		Program name. Default: teleport
	-f, --tar-file			Tar file where find the binaries
	-r, --build-release		Release number of the package
	-v, --build-version		Version number of the package
	-c, --changelog			Message to display in the changelog
					field of the RPM
	-s, --spec-location		Specify the output location of the
					spec file. Default: ~/rpmbuild/SPECS
	-h, --help			Display this message


EOF
exit 1
}

#Build various variables needed for the spec creation
build_deps(){
	ROOT_FOLDER="opt"
	BASE_INSTALL="\/$ROOT_FOLDER\/$P_NAME\/"
	TAR_FILE=$(echo $TAR_PKG | sed 's/.*\///')
	if [ ! -d "$HOME/rpmbuild" ]; then	
		rpmdev-setuptree
	fi
	SPEC_FILE="$SPEC_LOCATION$P_NAME$BUILD_VERSION-$BUILD_RELEASE.spec"
	# Remaking the TAR FILE
	tar xfz $TAR_PKG
	mkdir $ROOT_FOLDER
	cp -R $HOME/init-scripts $P_NAME
	mv $P_NAME $ROOT_FOLDER
	mkdir $P_NAME-$BUILD_VERSION-$BUILD_RELEASE
	mv $ROOT_FOLDER $P_NAME-$BUILD_VERSION-$BUILD_RELEASE
	cp -R $HOME/etc $P_NAME-$BUILD_VERSION-$BUILD_RELEASE/
	tar cfz $TAR_PKG $ROOT_FOLDER $P_NAME-$BUILD_VERSION-$BUILD_RELEASE 2&>/dev/null
	# Copy the source into the expected folder
	cp $TAR_PKG $HOME/rpmbuild/SOURCES/
	# Creating file list
	FILE_LIST=$(tar tf $TAR_PKG | cut -d '/' -f2,3,4,5,6,7,8,9,10 | grep -v $P_NAME.yaml | grep -v README.md | sed 's/^/\//' | grep -v "/$")
}

build_spec(){
SPEC_FILE="$SPEC_LOCATION$P_NAME$BUILD_VERSION-$BUILD_RELEASE.spec"
build_deps
cat<<EOXF >$SPEC_FILE
Name:           $P_NAME
Version:        $BUILD_VERSION
Release:        $BUILD_RELEASE
Summary:        SSH server with built-in bastion and a web UI

License:        Apache 2.0
URL:            http://gravitational.com/$P_NAME
Source0:        $TAR_FILE
Group:		Gravitational Inc <info@gravitational.com>

BuildArch:	x86_64
BuildRoot:	%{_tmppath}/\${name}%{version}
Requires: libc.so.6()(64bit) libc.so.6(GLIBC_2.2.5)(64bit) libpthread.so.0()(64bit) libpthread.so.0(GLIBC_2.2.5)(64bit) libpthread.so.0(GLIBC_2.3.2)(64bit) rtld(GNU_HASH)

%description
The SSH server with 2nd factor authentication, session recording and replay, Google Accounts integration and cluster membership introspection. It has a built-in SSH bastion and a beautiful Web UI.

%prep
if [ -f /etc/$P_NAME.yaml ]; then
	cp /etc/$P_NAME.yaml /tmp/$P_NAME.yaml.user
fi

%setup -q -n $P_NAME-$BUILD_VERSION-$BUILD_RELEASE

%install
mkdir -p "\$RPM_BUILD_ROOT"
cp -R * "\$RPM_BUILD_ROOT/"

%files
%config /etc/$P_NAME.yaml
%doc /opt/$P_NAME/README.md
$FILE_LIST

%post
ln -s /opt/$P_NAME/$P_NAME /usr/bin/
ln -s /opt/$P_NAME/tsh /usr/bin/
ln -s /opt/$P_NAME/tctl /usr/bin/
if [ ! -d /etc/$P_NAME ]; then 
	mkdir /etc/$P_NAME
fi
if [ -d /etc/systemd ]; then
	cp /opt/$P_NAME/init-scripts/systemd/system/$P_NAME.service /etc/systemd/system/$P_NAME.service
	systemctl daemon-reload
	systemctl start $P_NAME
	systemctl enable $P_NAME
elif [ -d /etc/init.d ]; then
	cp /opt/$P_NAME/init-scripts/init.d/$P_NAME /etc/init.d/$P_NAME
	service $P_NAME start
	chkconfig $P_NAME on
fi
if [ -f /tmp/$P_NAME.yaml ]; then
	mv /etc/$P_NAME.yaml /etc/$P_NAME.yaml.new
	mv /tmp/$P_NAME.yaml /etc/$P_NAME.yaml
fi
cat <<EOF 2>&1

########################
#Enjoy $P_NAME!
#
#If you need to generate the certificates for the proxy, you can execute
# openssl req -x509 -sha256 -nodes -days 365 -newkey rsa:2048 -keyout /etc/$P_NAME/$P_NAME.key -out /etc/$P_NAME/$P_NAME.crt
#*For more information read README.md
########################

EOF

%postun
rm -f /usr/bin/$P_NAME
rm -f /usr/bin/tsh
rm -f /usr/bin/tctl

%clean
rm -rf $RPM_BUILD_ROOD
%changelog
$CHANGE_LOG

EOXF
}

# Read the input options
TEMP=`getopt -o f:v:r:c:s:n: --long tar-file:,build-version:,build-release:,help,changelog:,spec-location:,program-name: -n 'parse-options' -- "$@"`
if [[ $? -ne 0 ]]; then
        usage
fi
eval set -- "$TEMP"
while [ $# -gt 0 ]; do
   case "$1" in
        -f | --tar-file ) TAR_PKG=$2 ; shift;shift ;;
        -v | --build-version ) BUILD_VERSION=$2; shift;shift ;;
        -r | --build-release ) BUILD_RELEASE=$2; shift;shift ;;
        -c | --changelog ) CHANGE_LOG=$2;  shift; shift;;
        -s | --spec-location ) SPEC_LOCATION=$2; shift; shift;;
	-n | --program-name ) P_NAME=$2; shift; shift;;
        -h | --help ) usage; shift; exit 1;;
        -- ) shift; break;;
        *) echo "Internal error!" ; usage| tee ~/error.log ; shift; exit 1 ;;
    esac
done

# Check vars
if [ -z "$TAR_PKG" ] || [ -z "$BUILD_VERSION" ] || [ -z "$BUILD_RELEASE" ]; then
        usage
fi
if [ -z "$SPEC_LOCATION" ]; then
        SPEC_LOCATION="$HOME/rpmbuild/SPECS/"
fi
if [ -z "$P_NAME" ]; then
	P_NAME="teleport"
fi

build_spec
rpmbuild --clean -ba $SPEC_FILE 

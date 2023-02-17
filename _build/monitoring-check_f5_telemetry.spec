#
# spec file for package monitoring-check_f5_telemetry
#
Name:           monitoring-check_f5_telemetry
Version:        %{version}
Release:        %{release}
Summary:        Icinga2/Nagios check for f5 telemetry data stored in Elasticsearch
License:        BSD
Group:          Sytem/Utilities
Vendor:         Ott-Consult UG
Packager:       Joern Ott
Url:            https://github.com/joernott/monitoring-check_f5_telemetry
Source0:        monitoring-check_f5_telemetry-%{version}.tar.gz
BuildArch:      x86_64

%description
A check for Icinga2 or Nagios to check f5 telemetry data stored in Elasticsearch

%prep
cd "$RPM_BUILD_DIR"
rm -rf *
tar -xzf "%{SOURCE0}"
STATUS=$?
if [ $STATUS -ne 0 ]; then
  exit $STATUS
fi
/usr/bin/chmod -Rf a+rX,u+w,g-w,o-w .

%build
cd "$RPM_BUILD_DIR/monitoring-check_f5_telemetry-%{version}/check_f5_telemetry"
go get -u -v
go build -v

%install
install -Dpm 0755 %{name}-%{version}/check_f5_telemetry/check_f5_telemetry "%{buildroot}/usr/lib64/nagios/plugins/check_f5_telemetry"

%files
%defattr(-,root,root,755)
%attr(755, root, root) /usr/lib64/nagios/plugins/check_f5_telemetry

%changelog
* Wed Feb 15 2023 Joern Ott <joern.ott@ott-consult.de>
- Initial version
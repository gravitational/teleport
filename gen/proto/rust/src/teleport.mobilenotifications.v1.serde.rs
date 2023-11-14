// @generated
impl serde::Serialize for RegisterDeviceRequest {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.device_token.is_empty() {
            len += 1;
        }
        if !self.cluster_id.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("teleport.mobilenotifications.v1.RegisterDeviceRequest", len)?;
        if !self.device_token.is_empty() {
            struct_ser.serialize_field("deviceToken", &self.device_token)?;
        }
        if !self.cluster_id.is_empty() {
            struct_ser.serialize_field("clusterId", &self.cluster_id)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for RegisterDeviceRequest {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "device_token",
            "deviceToken",
            "cluster_id",
            "clusterId",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            DeviceToken,
            ClusterId,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "deviceToken" | "device_token" => Ok(GeneratedField::DeviceToken),
                            "clusterId" | "cluster_id" => Ok(GeneratedField::ClusterId),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = RegisterDeviceRequest;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct teleport.mobilenotifications.v1.RegisterDeviceRequest")
            }

            fn visit_map<V>(self, mut map: V) -> std::result::Result<RegisterDeviceRequest, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut device_token__ = None;
                let mut cluster_id__ = None;
                while let Some(k) = map.next_key()? {
                    match k {
                        GeneratedField::DeviceToken => {
                            if device_token__.is_some() {
                                return Err(serde::de::Error::duplicate_field("deviceToken"));
                            }
                            device_token__ = Some(map.next_value()?);
                        }
                        GeneratedField::ClusterId => {
                            if cluster_id__.is_some() {
                                return Err(serde::de::Error::duplicate_field("clusterId"));
                            }
                            cluster_id__ = Some(map.next_value()?);
                        }
                    }
                }
                Ok(RegisterDeviceRequest {
                    device_token: device_token__.unwrap_or_default(),
                    cluster_id: cluster_id__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("teleport.mobilenotifications.v1.RegisterDeviceRequest", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for RegisterDeviceResponse {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.device_uuid.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("teleport.mobilenotifications.v1.RegisterDeviceResponse", len)?;
        if !self.device_uuid.is_empty() {
            struct_ser.serialize_field("deviceUuid", &self.device_uuid)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for RegisterDeviceResponse {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "device_uuid",
            "deviceUuid",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            DeviceUuid,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "deviceUuid" | "device_uuid" => Ok(GeneratedField::DeviceUuid),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = RegisterDeviceResponse;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct teleport.mobilenotifications.v1.RegisterDeviceResponse")
            }

            fn visit_map<V>(self, mut map: V) -> std::result::Result<RegisterDeviceResponse, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut device_uuid__ = None;
                while let Some(k) = map.next_key()? {
                    match k {
                        GeneratedField::DeviceUuid => {
                            if device_uuid__.is_some() {
                                return Err(serde::de::Error::duplicate_field("deviceUuid"));
                            }
                            device_uuid__ = Some(map.next_value()?);
                        }
                    }
                }
                Ok(RegisterDeviceResponse {
                    device_uuid: device_uuid__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("teleport.mobilenotifications.v1.RegisterDeviceResponse", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SendNotificationRequest {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.title.is_empty() {
            len += 1;
        }
        if !self.body.is_empty() {
            len += 1;
        }
        if !self.device_uuid.is_empty() {
            len += 1;
        }
        if !self.cluster_id.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("teleport.mobilenotifications.v1.SendNotificationRequest", len)?;
        if !self.title.is_empty() {
            struct_ser.serialize_field("title", &self.title)?;
        }
        if !self.body.is_empty() {
            struct_ser.serialize_field("body", &self.body)?;
        }
        if !self.device_uuid.is_empty() {
            struct_ser.serialize_field("deviceUuid", &self.device_uuid)?;
        }
        if !self.cluster_id.is_empty() {
            struct_ser.serialize_field("clusterId", &self.cluster_id)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SendNotificationRequest {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "title",
            "body",
            "device_uuid",
            "deviceUuid",
            "cluster_id",
            "clusterId",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Title,
            Body,
            DeviceUuid,
            ClusterId,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "title" => Ok(GeneratedField::Title),
                            "body" => Ok(GeneratedField::Body),
                            "deviceUuid" | "device_uuid" => Ok(GeneratedField::DeviceUuid),
                            "clusterId" | "cluster_id" => Ok(GeneratedField::ClusterId),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = SendNotificationRequest;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct teleport.mobilenotifications.v1.SendNotificationRequest")
            }

            fn visit_map<V>(self, mut map: V) -> std::result::Result<SendNotificationRequest, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut title__ = None;
                let mut body__ = None;
                let mut device_uuid__ = None;
                let mut cluster_id__ = None;
                while let Some(k) = map.next_key()? {
                    match k {
                        GeneratedField::Title => {
                            if title__.is_some() {
                                return Err(serde::de::Error::duplicate_field("title"));
                            }
                            title__ = Some(map.next_value()?);
                        }
                        GeneratedField::Body => {
                            if body__.is_some() {
                                return Err(serde::de::Error::duplicate_field("body"));
                            }
                            body__ = Some(map.next_value()?);
                        }
                        GeneratedField::DeviceUuid => {
                            if device_uuid__.is_some() {
                                return Err(serde::de::Error::duplicate_field("deviceUuid"));
                            }
                            device_uuid__ = Some(map.next_value()?);
                        }
                        GeneratedField::ClusterId => {
                            if cluster_id__.is_some() {
                                return Err(serde::de::Error::duplicate_field("clusterId"));
                            }
                            cluster_id__ = Some(map.next_value()?);
                        }
                    }
                }
                Ok(SendNotificationRequest {
                    title: title__.unwrap_or_default(),
                    body: body__.unwrap_or_default(),
                    device_uuid: device_uuid__.unwrap_or_default(),
                    cluster_id: cluster_id__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("teleport.mobilenotifications.v1.SendNotificationRequest", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for SendNotificationResponse {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let len = 0;
        let struct_ser = serializer.serialize_struct("teleport.mobilenotifications.v1.SendNotificationResponse", len)?;
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for SendNotificationResponse {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                            Err(serde::de::Error::unknown_field(value, FIELDS))
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = SendNotificationResponse;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct teleport.mobilenotifications.v1.SendNotificationResponse")
            }

            fn visit_map<V>(self, mut map: V) -> std::result::Result<SendNotificationResponse, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                while map.next_key::<GeneratedField>()?.is_some() {
                    let _ = map.next_value::<serde::de::IgnoredAny>()?;
                }
                Ok(SendNotificationResponse {
                })
            }
        }
        deserializer.deserialize_struct("teleport.mobilenotifications.v1.SendNotificationResponse", FIELDS, GeneratedVisitor)
    }
}

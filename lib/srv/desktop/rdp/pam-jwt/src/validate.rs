use base64::prelude::BASE64_URL_SAFE_NO_PAD;
use base64::Engine;
use jsonwebtoken::jwk::{Jwk, JwkSet};
use jsonwebtoken::{Algorithm, DecodingKey, Validation};
use reqwest::blocking::Client;
use serde::{Deserialize, Serialize};
use std::error::Error;
use std::fmt::{Display, Formatter};
use std::str::FromStr;
use std::time::Duration;

const JWKS_WELL_KNOWN_PATH: &str = "/.well-known/jwks.json";

#[derive(Debug, Serialize, Deserialize)]
struct Claims {
    sub: String,
}

#[derive(Debug, Serialize, Deserialize)]
struct Header {
    alg: Algorithm,
    #[serde(skip_serializing_if = "Option::is_none")]
    kid: Option<String>,
    typ: String,
}

pub(super) fn validate(
    username: &str,
    jwt: &str,
    url: Option<&&str>,
    file: Option<&&str>,
    skip_verify: bool,
) -> Result<(), Box<dyn Error>> {
    println!("getting jwks from url");
    let jwk_url = url
        .map(|url| get_jwks_url(url, skip_verify))
        .transpose()?
        .map(|j| j.keys)
        .unwrap_or_default();
    println!("getting jwks from file");
    let jwk_file = file
        .map(get_jwks_file)
        .transpose()?
        .map(|j| j.keys)
        .unwrap_or_default();

    let need_header = need_header(jwt);

    for jwk in jwk_url.iter().chain(jwk_file.iter()) {
        match validate_single(username, jwt, jwk, need_header) {
            Ok(_) => {
                return Ok(());
            }
            Err(e) => {
                println!("Invalid token {e:?}");
            }
        };
    }

    Err(NoMatchingKeysError.into())
}

fn validate_single(
    username: &str,
    jwt: &str,
    jwk: &Jwk,
    need_header: bool,
) -> Result<(), Box<dyn Error>> {
    let algorithm = Algorithm::from_str(&format!("{}", jwk.common.key_algorithm.unwrap()))?;
    let jwt = if need_header {
        let key_id = jwk.common.key_id.clone();
        let header = generate_header(algorithm, key_id);
        format!("{header}.{jwt}")
    } else {
        jwt.to_string()
    };

    let mut validation = Validation::new(algorithm);
    validation.sub = Some(username.to_string());
    validation.set_audience(&["x"]);
    let token =
        jsonwebtoken::decode::<Claims>(&jwt, &DecodingKey::from_jwk(jwk).unwrap(), &validation)?;

    println!("Validated token: {token:?}");

    Ok(())
}

fn generate_header(algorithm: Algorithm, key_id: Option<String>) -> String {
    let header = Header {
        alg: algorithm,
        kid: key_id,
        typ: "JWT".to_string(),
    };
    let header = serde_json::to_vec(&header).unwrap();
    BASE64_URL_SAFE_NO_PAD.encode(header)
}

fn need_header(jwt: &str) -> bool {
    jwt.chars().filter(|&c| c == '.').count() < 2
}

#[derive(Debug, Clone)]
struct NoMatchingKeysError;

impl Display for NoMatchingKeysError {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        write!(f, "no matching keys found")
    }
}

impl Error for NoMatchingKeysError {}

fn get_jwks_url(url: &&str, skip_verify: bool) -> reqwest::Result<JwkSet> {
    let client = Client::builder()
        .danger_accept_invalid_certs(skip_verify)
        .danger_accept_invalid_hostnames(skip_verify)
        .timeout(Duration::from_secs(2))
        .build()?;
    println!("reqwest");
    let response = client.get(format!("{url}{JWKS_WELL_KNOWN_PATH}")).send()?;
    println!("{response:?}");
    response.error_for_status()?.json()
}

fn get_jwks_file(path: &&str) -> Result<JwkSet, Box<dyn Error>> {
    Ok(serde_json::from_reader(std::fs::File::open(path)?)?)
}

#[cfg(test)]
mod tests {
    use super::*;
    use httpmock::prelude::*;
    use jsonwebtoken::jwk::KeyAlgorithm;

    fn jwks_json_path() -> String {
        format!("{}/jwks.json", env!("CARGO_MANIFEST_DIR"))
    }

    #[test]
    fn test_get_jwks_file() -> Result<(), Box<dyn Error>> {
        let path = jwks_json_path();
        let jwks = get_jwks_file(&path.as_str())?;
        assert_eq!(jwks.keys.len(), 2);
        assert_eq!(jwks.keys[0].common.key_algorithm, Some(KeyAlgorithm::ES256));
        assert_eq!(
            jwks.keys[0].common.key_id,
            Some("elbgrs83NageOe5zH4un8pJpcbLXKrvLAdh1YRK9GPU".to_string())
        );

        Ok(())
    }

    #[test]
    fn test_get_jwks_url() -> Result<(), Box<dyn Error>> {
        let server = MockServer::start();
        let mock = server.mock(|when, then| {
            when.path(JWKS_WELL_KNOWN_PATH);
            then.body_from_file(jwks_json_path());
        });
        let jwks = get_jwks_url(&server.url("").as_str(), true)?;

        mock.assert();

        assert_eq!(jwks.keys.len(), 2);
        assert_eq!(jwks.keys[0].common.key_algorithm, Some(KeyAlgorithm::ES256));
        assert_eq!(
            jwks.keys[0].common.key_id,
            Some("elbgrs83NageOe5zH4un8pJpcbLXKrvLAdh1YRK9GPU".to_string())
        );

        Ok(())
    }

    const TOKEN_WITH_HEADER: &str = "eyJhbGciOiJFUzI1NiIsImtpZCI6ImVsYmdyczgzTmFnZU9lNXpINHVuOHBKcGNiTFhLcnZMQWRoMVlSSzlHUFUiLCJ0eXAiOiJKV1QifQ.eyJhdWQiOiJ4IiwiZXhwIjo0OTExNTcxODk2LCJpYXQiOjE3NTc5NzE4OTYsImlzcyI6InByb3h5LmRvbS5pdCIsIm5iZiI6MTc1Nzk3MTg4Niwic3ViIjoibmV3dXNlciIsInVzZXJuYW1lIjoibmV3dXNlciJ9.7qdi5yUJfGPuY_8vWtju-6DNK5cbfIUU8OEeMuE2HVJuJX9GKOtigQ7J9kSFN9wz-upFEKSuEy8H831Qmg8BjQ";
    const TOKEN_WITHOUT_HEADER: &str = "eyJhdWQiOiJ4IiwiZXhwIjo0OTExNTcxODk2LCJpYXQiOjE3NTc5NzE4OTYsImlzcyI6InByb3h5LmRvbS5pdCIsIm5iZiI6MTc1Nzk3MTg4Niwic3ViIjoibmV3dXNlciIsInVzZXJuYW1lIjoibmV3dXNlciJ9.7qdi5yUJfGPuY_8vWtju-6DNK5cbfIUU8OEeMuE2HVJuJX9GKOtigQ7J9kSFN9wz-upFEKSuEy8H831Qmg8BjQ";

    #[test]
    fn test_validate_single() -> Result<(), Box<dyn Error>> {
        let jwk = &get_jwks_file(&jwks_json_path().as_str())?.keys[0];
        validate_single("newuser", TOKEN_WITH_HEADER, jwk, false)?;
        validate_single("newuser", TOKEN_WITHOUT_HEADER, jwk, true)?;
        Ok(())
    }

    #[test]
    fn test_generate_header() -> Result<(), Box<dyn Error>> {
        let header = generate_header(
            Algorithm::ES256,
            Some("elbgrs83NageOe5zH4un8pJpcbLXKrvLAdh1YRK9GPU".to_string()),
        );
        assert_eq!(&header, "eyJhbGciOiJFUzI1NiIsImtpZCI6ImVsYmdyczgzTmFnZU9lNXpINHVuOHBKcGNiTFhLcnZMQWRoMVlSSzlHUFUiLCJ0eXAiOiJKV1QifQ");
        Ok(())
    }
}

import React from 'react';
import { Basic, Success, Failed } from './Login.story';
import { convertToJSON } from 'design/utils/testing';

describe('Gravity/Login.story', () => {
  it('basic login renders correctly', () => {
    const tree = convertToJSON(<Basic />);

    expect(tree).toMatchInlineSnapshot(`
      Array [
        <img
          className="sc-bdVaJa kaMnFS"
          src="file_stub"
        />,
        <form
          className="sc-htpNat sc-htoDjs giGZdH"
          width="456px"
        >
          <div
            className="sc-htpNat kyeECs"
          >
            <div
              className="sc-fjdhpX rqkSz"
              color="light"
            >
              Gravity
            </div>
            <div
              className="sc-htpNat dfbocc"
            >
              <label
                className="sc-gqjmRU gKhUOi"
                fontSize={0}
              >
                Username
              </label>
              <input
                autoComplete="off"
                autoFocus={true}
                className="sc-iwsKbI hMKqPV"
                color="text.onLight"
                onChange={[Function]}
                placeholder="User name"
                value=""
              />
            </div>
            <div
              className="sc-htpNat dfbocc"
            >
              <label
                className="sc-gqjmRU gKhUOi"
                fontSize={0}
              >
                Password
              </label>
              <input
                autoComplete="off"
                className="sc-iwsKbI hMKqPV"
                color="text.onLight"
                onChange={[Function]}
                placeholder="Password"
                type="password"
                value=""
              />
            </div>
            <button
              className="sc-bxivhb hkyxUK"
              kind="primary"
              onClick={[Function]}
              size="large"
              type="submit"
              width="100%"
            >
              LOGIN
            </button>
          </div>
        </form>,
      ]
    `);
  });

  it('login success renders correctly', () => {
    const tree = convertToJSON(<Success />);
    expect(tree).toMatchInlineSnapshot(`
      Array [
        <img
          className="sc-bdVaJa kaMnFS"
          src="file_stub"
        />,
        <div
          className="sc-htpNat sc-htoDjs iFmlWo"
          width="540px"
        >
          <span
            className="icon icon-checkmark-circle  sc-ifAKCX fntpuv"
            color="success"
            fontSize={64}
          />
          <div
            className="sc-fjdhpX bYLuAl"
          >
            Login Successful
          </div>
          <div
            className="sc-fjdhpX gHGBk"
          >
            You have successfully signed into your account. You can close this window and continue using the product.
          </div>
        </div>,
      ]
    `);
  });

  it('failed login renders correctly', () => {
    const tree = convertToJSON(<Failed />);
    expect(tree).toMatchInlineSnapshot(`
      Array [
        <img
          className="sc-bdVaJa kaMnFS"
          src="file_stub"
        />,
        <div
          className="sc-htpNat sc-htoDjs jRHzQX"
          color="text.onLight"
          width="540px"
        >
          <div
            className="sc-fjdhpX elIsj"
          >
            Login unsuccessful
          </div>
          <div>
             
            <div
              className="sc-fjdhpX jdCZOL"
            >
              <div
                className="sc-fjdhpX gHGBk"
              >
                <a
                  className="sc-eNQAEJ bQnGl"
                  href="/web/login"
                >
                  Please try again
                </a>
                , if the problem persists, contact your system administrator.
              </div>
            </div>
          </div>
        </div>,
      ]
    `);
  });
});

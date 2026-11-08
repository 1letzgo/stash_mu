import React from "react";
import { ModalSetting, NumberSetting } from "./Inputs";
import { SettingSection } from "./SettingSection";
import * as GQL from "src/core/generated-graphql";
import { Button, Form } from "react-bootstrap";
import { useIntl } from "react-intl";
import { useSettings } from "./context";
import { LoadingIndicator } from "../Shared/LoadingIndicator";
import { useToast } from "src/hooks/Toast";
import { useGenerateAPIKey } from "src/core/StashService";
import { useCurrentUser } from "src/hooks/CurrentUser";
import { ModalComponent } from "../Shared/Modal";

const ChangePasswordSetting: React.FC = () => {
  const intl = useIntl();
  const Toast = useToast();
  const [show, setShow] = React.useState(false);
  const [current, setCurrent] = React.useState("");
  const [next, setNext] = React.useState("");
  const [confirm, setConfirm] = React.useState("");
  const [changePassword, { loading }] = GQL.useChangePasswordMutation();

  function close() {
    setShow(false);
    setCurrent("");
    setNext("");
    setConfirm("");
  }

  async function onSave() {
    try {
      await changePassword({
        variables: { input: { current_password: current, new_password: next } },
      });
      Toast.success(
        intl.formatMessage({ id: "config.users.password_changed" })
      );
      close();
    } catch (e) {
      Toast.error(e);
    }
  }

  return (
    <div className="setting" id="change-password">
      <div>
        <h3>{intl.formatMessage({ id: "config.users.change_password" })}</h3>
      </div>
      <div>
        <Button onClick={() => setShow(true)}>
          {intl.formatMessage({ id: "config.users.change_password" })}
        </Button>
      </div>
      <ModalComponent
        show={show}
        header={intl.formatMessage({ id: "config.users.change_password" })}
        accept={{
          text: intl.formatMessage({ id: "actions.save" }),
          onClick: onSave,
        }}
        cancel={{
          text: intl.formatMessage({ id: "actions.cancel" }),
          variant: "secondary",
          onClick: close,
        }}
        onHide={close}
        isRunning={loading}
        disabled={!current || !next || next !== confirm}
      >
        <Form.Group>
          <Form.Label>
            {intl.formatMessage({ id: "config.users.current_password" })}
          </Form.Label>
          <Form.Control
            className="text-input"
            type="password"
            autoComplete="current-password"
            value={current}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setCurrent(e.currentTarget.value)
            }
          />
        </Form.Group>
        <Form.Group>
          <Form.Label>
            {intl.formatMessage({ id: "config.users.new_password" })}
          </Form.Label>
          <Form.Control
            className="text-input"
            type="password"
            autoComplete="new-password"
            value={next}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setNext(e.currentTarget.value)
            }
          />
        </Form.Group>
        <Form.Group>
          <Form.Label>
            {intl.formatMessage({ id: "config.users.confirm_password" })}
          </Form.Label>
          <Form.Control
            className="text-input"
            type="password"
            autoComplete="new-password"
            value={confirm}
            isInvalid={confirm !== "" && confirm !== next}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setConfirm(e.currentTarget.value)
            }
          />
        </Form.Group>
      </ModalComponent>
    </div>
  );
};

type AuthenticationSettingsInput = Pick<
  GQL.ConfigGeneralInput,
  "username" | "password"
>;

interface IAuthenticationInput {
  value: AuthenticationSettingsInput;
  setValue: (v: AuthenticationSettingsInput) => void;
}

const AuthenticationInput: React.FC<IAuthenticationInput> = ({
  value,
  setValue,
}) => {
  const intl = useIntl();

  function set(v: Partial<AuthenticationSettingsInput>) {
    setValue({
      ...value,
      ...v,
    });
  }

  const { username, password } = value;

  return (
    <div>
      <Form.Group id="username">
        <h6>{intl.formatMessage({ id: "config.general.auth.username" })}</h6>
        <Form.Control
          className="text-input"
          value={username ?? ""}
          onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
            set({ username: e.currentTarget.value })
          }
        />
        <Form.Text className="text-muted">
          {intl.formatMessage({ id: "config.general.auth.username_desc" })}
        </Form.Text>
      </Form.Group>
      <Form.Group id="password">
        <h6>{intl.formatMessage({ id: "config.general.auth.password" })}</h6>
        <Form.Control
          className="text-input"
          type="password"
          value={password ?? ""}
          onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
            set({ password: e.currentTarget.value })
          }
        />
        <Form.Text className="text-muted">
          {intl.formatMessage({ id: "config.general.auth.password_desc" })}
        </Form.Text>
      </Form.Group>
    </div>
  );
};

export const SettingsSecurityPanel: React.FC = () => {
  const intl = useIntl();
  const Toast = useToast();

  const { general, apiKey, loading, error, saveGeneral, refetch } =
    useSettings();

  const [generateAPIKey] = useGenerateAPIKey();
  const { isAdmin, multiUser, me } = useCurrentUser();

  async function onGenerateAPIKey() {
    try {
      await generateAPIKey({
        variables: {
          input: {},
        },
      });
      refetch();
    } catch (e) {
      Toast.error(e);
    }
  }

  async function onClearAPIKey() {
    try {
      await generateAPIKey({
        variables: {
          input: {
            clear: true,
          },
        },
      });
      refetch();
    } catch (e) {
      Toast.error(e);
    }
  }

  if (error) return <h1>{error.message}</h1>;
  if (loading) return <LoadingIndicator />;

  return (
    <SettingSection headingID="config.general.auth.authentication">
      {multiUser && (
        <div className="setting" id="current-user">
          <div>
            <h3>{intl.formatMessage({ id: "config.users.signed_in_as" })}</h3>
            <div className="value">
              {me?.username} (
              {intl.formatMessage({
                id: `config.users.roles.${me?.role.toLowerCase()}`,
              })}
              )
            </div>
          </div>
          <div />
        </div>
      )}
      {multiUser && <ChangePasswordSetting />}
      {!multiUser && (
        <ModalSetting<AuthenticationSettingsInput>
          id="authentication-settings"
          headingID="config.general.auth.credentials.heading"
          subHeadingID="config.general.auth.credentials.description"
          value={{
            username: general.username,
            password: general.password,
          }}
          onChange={(v) => saveGeneral(v)}
          renderField={(value, setValue) => (
            <AuthenticationInput value={value ?? {}} setValue={setValue} />
          )}
          renderValue={(v) => {
            if (v?.username && v?.password)
              return <span>{v?.username ?? ""}</span>;
            return null;
          }}
        />
      )}

      <div className="setting" id="apikey">
        <div>
          <h3>{intl.formatMessage({ id: "config.general.auth.api_key" })}</h3>

          <div className="value text-break">{apiKey}</div>

          <div className="sub-heading">
            {intl.formatMessage({ id: "config.general.auth.api_key_desc" })}
          </div>
        </div>
        <div>
          <Button
            disabled={!general.username || !general.password}
            onClick={() => onGenerateAPIKey()}
          >
            {intl.formatMessage({
              id: "config.general.auth.generate_api_key",
            })}
          </Button>
          <Button variant="danger" onClick={() => onClearAPIKey()}>
            {intl.formatMessage({
              id: "config.general.auth.clear_api_key",
            })}
          </Button>
        </div>
      </div>

      {isAdmin && (
        <NumberSetting
          id="maxSessionAge"
          headingID="config.general.auth.maximum_session_age"
          subHeadingID="config.general.auth.maximum_session_age_desc"
          value={general.maxSessionAge ?? undefined}
          onChange={(v) => saveGeneral({ maxSessionAge: v })}
        />
      )}
    </SettingSection>
  );
};

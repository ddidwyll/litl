{ config, pkgs, lib, ... }:

let
  cfg = config.services.litl;
in
{
  options.services.litl = {
    enable = lib.mkEnableOption "LITL daemon";
    package = lib.mkOption {
      type = lib.types.package;
      default = pkgs.litl;
    };
    port = lib.mkOption {
      type = lib.types.int;
      default = 8765;
    };
    authToken = lib.mkOption {
      type = lib.types.str;
      description = "Auth token for API";
    };
  };

  config = lib.mkIf cfg.enable {
    systemd.user.services.litl = {
      Unit = {
        Description = "LITL daemon";
        After = [ "network.target" ];
      };
      Service = {
        ExecStart = "${cfg.package}/bin/ai-daemon --port ${toString cfg.port}";
        Restart = "on-failure";
        Environment = [
          "LITL_AUTH_TOKEN=${cfg.authToken}"
          "ROUTER_API_KEY=\${HOME}/.config/litl/api_key"
        ];
      };
      Install.WantedBy = [ "default.target" ];
    };
  };
}
